package api

import (
	"net/http"
	"testing"
	"time"

	"dockeradmin/internal/model"
)

func TestAlertEventsLimitReturnsNewest(t *testing.T) {
	r, store := newTestServer(t)
	older := time.Now().Add(-time.Minute)
	newer := older.Add(30 * time.Second)
	store.AddEvent(model.AlertEvent{RuleID: "rule-1", RuleName: "older", Kind: "fired", FiredAt: older})
	store.AddEvent(model.AlertEvent{RuleID: "rule-1", RuleName: "newer", Kind: "recovered", FiredAt: newer})

	w, body := doJSON(t, r, http.MethodGet, "/api/alert-events?limit=1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("event list status = %d, want %d", w.Code, http.StatusOK)
	}
	events, ok := body["data"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("event list = %#v, want exactly one event", body["data"])
	}
	got := events[0].(map[string]any)
	if got["kind"] != "recovered" || got["rule_name"] != "newer" {
		t.Fatalf("latest event = kind %v, rule_name %v; want recovered/newer", got["kind"], got["rule_name"])
	}
}
