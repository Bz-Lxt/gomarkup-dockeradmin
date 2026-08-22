package api

import (
	"net/http"
	"testing"
)

func TestAlertRuleCreateAcceptsMaximumDuration(t *testing.T) {
	r, _ := newTestServer(t)
	payload := `{"name":"全天持续告警","metric":"cpu_percent","op":">","threshold":80,"duration_sec":86400,"cooldown_sec":300,"enabled":true,"webhook_url":"http://localhost:8080/api/mock/webhook"}`

	w, body := doJSON(t, r, http.MethodPost, "/api/alert-rules", payload)
	if w.Code != http.StatusCreated {
		t.Fatalf("duration_sec=86400 should be accepted, status = %d, body = %v", w.Code, body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["duration_sec"] != float64(86400) {
		t.Fatalf("created rule duration_sec = %v, want 86400", data["duration_sec"])
	}
}
