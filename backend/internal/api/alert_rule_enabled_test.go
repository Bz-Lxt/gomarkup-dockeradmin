package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestAlertRuleUpdateCanDisableRule(t *testing.T) {
	r, _ := newTestServer(t)
	createBody := `{"name":"CPU 过高","metric":"cpu_percent","op":">","threshold":80,"duration_sec":30,"cooldown_sec":300,"enabled":true,"webhook_url":"http://localhost:8080/hook"}`
	w, body := doJSON(t, r, http.MethodPost, "/api/alert-rules", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %v", w.Code, http.StatusCreated, body)
	}
	id := body["data"].(map[string]any)["id"].(string)

	updateBody := strings.Replace(createBody, `"enabled":true`, `"enabled":false`, 1)
	w, body = doJSON(t, r, http.MethodPut, "/api/alert-rules/"+id, updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d: %v", w.Code, http.StatusOK, body)
	}
	if enabled := body["data"].(map[string]any)["enabled"]; enabled != false {
		t.Fatalf("updated enabled = %v, want false", enabled)
	}

	w, body = doJSON(t, r, http.MethodGet, "/api/alert-rules", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %v", w.Code, http.StatusOK, body)
	}
	rules := body["data"].([]any)
	if len(rules) != 1 {
		t.Fatalf("rule count = %d, want 1", len(rules))
	}
	if enabled := rules[0].(map[string]any)["enabled"]; enabled != false {
		t.Fatalf("listed enabled = %v, want false", enabled)
	}
}
