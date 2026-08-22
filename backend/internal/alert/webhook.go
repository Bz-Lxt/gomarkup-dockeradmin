package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"dockeradmin/internal/model"
)

// Notifier Webhook 通知器：5s 超时；窄重试 —— 仅网络错误与 5xx 重试 1 次，4xx 不重试。
type Notifier struct {
	client *http.Client
	log    *slog.Logger
}

type NotifyResult struct {
	Status int
	Err    string
}

func NewNotifier(log *slog.Logger) *Notifier {
	return &Notifier{
		client: &http.Client{Timeout: 5 * time.Second},
		log:    log,
	}
}

func (n *Notifier) Send(ctx context.Context, webhookURL string, payload model.WebhookPayload) NotifyResult {
	body, err := json.Marshal(payload)
	if err != nil {
		return NotifyResult{Err: fmt.Sprintf("marshal payload: %v", err)}
	}

	var last NotifyResult
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return last
			case <-time.After(500 * time.Millisecond):
			}
		}
		last = n.sendOnce(ctx, webhookURL, body)
		// 窄重试判定：网络错误（Status=0）或 5xx 才重试；4xx 属客户端错误，重试无意义
		if last.Err == "" || (last.Status > 0 && last.Status < 500) {
			return last
		}
		n.log.Debug("webhook transient failure, retrying once", "url", webhookURL, "status", last.Status, "err", last.Err)
	}
	return last
}

func (n *Notifier) sendOnce(ctx context.Context, webhookURL string, body []byte) NotifyResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return NotifyResult{Err: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DockerAdmin-Webhook/1.0")

	resp, err := n.client.Do(req)
	if err != nil {
		return NotifyResult{Err: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 400 {
		return NotifyResult{Status: resp.StatusCode, Err: fmt.Sprintf("webhook returned %d", resp.StatusCode)}
	}
	return NotifyResult{Status: resp.StatusCode}
}
