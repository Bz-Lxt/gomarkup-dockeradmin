package alert_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"dockeradmin/internal/alert"
	"dockeradmin/internal/model"
)

func TestNotifierDoesNotRetryClientErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	notifier := alert.NewNotifier(slog.Default())
	result := notifier.Send(context.Background(), server.URL, model.WebhookPayload{RuleID: "rule-1"})

	if result.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", result.Status, http.StatusTooManyRequests)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("webhook requests = %d, want 1 for a client error", got)
	}
}
