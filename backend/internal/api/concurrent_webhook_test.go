package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"dockeradmin/internal/model"
)

func TestMockWebhookConcurrentReceipts(t *testing.T) {
	router, _ := newTestServer(t)

	const requests = 64
	start := make(chan struct{})
	errCh := make(chan error, requests)
	ids := make(chan int64, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"delivery":%d}`, i)
			req := httptest.NewRequest(http.MethodPost, "/api/mock/webhook", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			<-start
			router.ServeHTTP(w, req)
			var result struct {
				Data struct {
					Received bool  `json:"received"`
					ID       int64 `json:"id"`
				} `json:"data"`
			}
			decodeErr := json.NewDecoder(w.Body).Decode(&result)
			if decodeErr != nil {
				errCh <- decodeErr
				return
			}
			if w.Code != http.StatusOK || !result.Data.Received {
				errCh <- fmt.Errorf("delivery %d: status=%d received=%v", i, w.Code, result.Data.Received)
				return
			}
			ids <- result.Data.ID
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(ids)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/mock/webhook/receipts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var result struct {
		Data []model.WebhookReceipt `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("receipt list status = %d, want 200", w.Code)
	}
	if len(result.Data) != requests {
		t.Fatalf("receipt count = %d, want %d successful deliveries", len(result.Data), requests)
	}

	wantIDs := make(map[int64]struct{}, requests)
	for id := range ids {
		wantIDs[id] = struct{}{}
	}
	if len(wantIDs) != requests {
		t.Fatalf("response IDs contain duplicates: got %d unique IDs, want %d", len(wantIDs), requests)
	}
	for _, receipt := range result.Data {
		if _, ok := wantIDs[receipt.ID]; !ok {
			t.Fatalf("receipt ID %d was not returned by a successful delivery", receipt.ID)
		}
		delete(wantIDs, receipt.ID)
	}
	if len(wantIDs) != 0 {
		t.Fatalf("%d successful delivery IDs are missing from receipts", len(wantIDs))
	}
}
