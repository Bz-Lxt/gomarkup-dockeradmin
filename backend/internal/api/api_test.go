package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dockeradmin/internal/alert"
	"dockeradmin/internal/collector"
	"dockeradmin/internal/config"
	"dockeradmin/internal/dockermon"
	"dockeradmin/internal/logger"
)

func newTestServer(t *testing.T) (*gin.Engine, *alert.Store) {
	t.Helper()
	cfg := &config.Config{Port: 8080, CollectInterval: 2 * time.Second, RetentionWindow: time.Hour, LogLevel: "error", Version: "test"}
	log := logger.New("error")
	col := collector.New(2*time.Second, time.Hour, "", log)
	dm := &dockermon.Monitor{} // 零值 = 降级模式（无 client）
	store, err := alert.NewStore(filepath.Join(t.TempDir(), "alerts.json"), log)
	if err != nil {
		t.Fatal(err)
	}
	return NewRouter(cfg, log, col, dm, store), store
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var parsed map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("%s %s: 响应非 JSON: %v", method, path, err)
		}
	}
	return w, parsed
}

func TestHealth(t *testing.T) {
	r, _ := newTestServer(t)
	w, body := doJSON(t, r, http.MethodGet, "/api/health", "")
	if w.Code != 200 {
		t.Fatalf("health status = %d", w.Code)
	}
	data := body["data"].(map[string]any)
	if data["status"] != "ok" || data["docker"] != "degraded" {
		t.Fatalf("health 字段错误: %v", data)
	}
}

func TestMetricsCurrent_NoData(t *testing.T) {
	r, _ := newTestServer(t)
	w, body := doJSON(t, r, http.MethodGet, "/api/metrics/current", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("无数据应 503，实际 %d", w.Code)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "no_data" {
		t.Fatalf("错误信封 code 错误: %v", errObj)
	}
}

func TestMetricsHistory_ParamValidation(t *testing.T) {
	r, _ := newTestServer(t)
	w, _ := doJSON(t, r, http.MethodGet, "/api/metrics/history?minutes=abc", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 minutes 应 400，实际 %d", w.Code)
	}
	w, body := doJSON(t, r, http.MethodGet, "/api/metrics/history?minutes=30", "")
	if w.Code != 200 {
		t.Fatalf("合法 minutes 应 200，实际 %d", w.Code)
	}
	if _, ok := body["data"].([]any); !ok {
		t.Fatal("history data 应为数组")
	}
}

func TestContainers_Degraded(t *testing.T) {
	r, _ := newTestServer(t)
	w, body := doJSON(t, r, http.MethodGet, "/api/containers", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("降级模式应 503，实际 %d", w.Code)
	}
	if body["error"].(map[string]any)["code"] != "docker_unavailable" {
		t.Fatalf("错误码应为 docker_unavailable: %v", body)
	}
}

func TestRuleCRUD(t *testing.T) {
	r, _ := newTestServer(t)

	// 创建（201 + Location）
	valid := `{"name":"CPU 过高","metric":"cpu_percent","op":">","threshold":80,"duration_sec":30,"cooldown_sec":300,"enabled":true,"webhook_url":"http://localhost:8080/api/mock/webhook","notify_recovery":true}`
	w, body := doJSON(t, r, http.MethodPost, "/api/alert-rules", valid)
	if w.Code != http.StatusCreated {
		t.Fatalf("创建应 201，实际 %d: %v", w.Code, body)
	}
	if w.Header().Get("Location") == "" {
		t.Fatal("201 应带 Location 头")
	}
	id := body["data"].(map[string]any)["id"].(string)

	// 非法创建（422 + details）
	w, body = doJSON(t, r, http.MethodPost, "/api/alert-rules", `{"name":"","metric":"cpu_percent","op":">","threshold":101,"webhook_url":"ftp://x"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("非法规则应 422，实际 %d", w.Code)
	}
	details := body["error"].(map[string]any)["details"].([]any)
	if len(details) < 3 {
		t.Fatalf("应报告 name/threshold/webhook_url 三处错误: %v", details)
	}

	// 非法 JSON（400）
	w, _ = doJSON(t, r, http.MethodPost, "/api/alert-rules", `{broken`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("坏 JSON 应 400，实际 %d", w.Code)
	}

	// 列表
	w, body = doJSON(t, r, http.MethodGet, "/api/alert-rules", "")
	if w.Code != 200 || len(body["data"].([]any)) != 1 {
		t.Fatalf("列表应有 1 条: %v", body)
	}

	// 更新
	w, body = doJSON(t, r, http.MethodPut, "/api/alert-rules/"+id, strings.Replace(valid, `"threshold":80`, `"threshold":90`, 1))
	if w.Code != 200 || body["data"].(map[string]any)["threshold"].(float64) != 90 {
		t.Fatalf("更新失败: %d %v", w.Code, body)
	}

	// 更新不存在（404）
	w, _ = doJSON(t, r, http.MethodPut, "/api/alert-rules/nope", valid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("更新不存在规则应 404，实际 %d", w.Code)
	}

	// 删除（204）+ 重复删除（404）
	w, _ = doJSON(t, r, http.MethodDelete, "/api/alert-rules/"+id, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("删除应 204，实际 %d", w.Code)
	}
	w, _ = doJSON(t, r, http.MethodDelete, "/api/alert-rules/"+id, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("重复删除应 404，实际 %d", w.Code)
	}
}

func TestMockWebhook(t *testing.T) {
	r, _ := newTestServer(t)
	w, body := doJSON(t, r, http.MethodPost, "/api/mock/webhook", `{"hello":"world"}`)
	if w.Code != 200 || body["data"].(map[string]any)["received"] != true {
		t.Fatalf("mock webhook 应接收成功: %d %v", w.Code, body)
	}
	w, body = doJSON(t, r, http.MethodGet, "/api/mock/webhook/receipts", "")
	if w.Code != 200 {
		t.Fatalf("receipts 应 200，实际 %d", w.Code)
	}
	receipts := body["data"].([]any)
	if len(receipts) != 1 || !strings.Contains(receipts[0].(map[string]any)["payload"].(string), "hello") {
		t.Fatalf("receipts 内容错误: %v", receipts)
	}
}

func TestSPAFallback(t *testing.T) {
	r, _ := newTestServer(t)
	// API 404 返回 JSON 错误信封
	w, body := doJSON(t, r, http.MethodGet, "/api/nonexistent", "")
	if w.Code != 404 || body["error"] == nil {
		t.Fatalf("API 404 应返回错误信封: %d %v", w.Code, body)
	}
}
