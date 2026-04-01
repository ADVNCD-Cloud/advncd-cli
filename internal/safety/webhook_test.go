package safety

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookMiddlewareRejectsQuerySecrets(t *testing.T) {
	t.Parallel()

	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	h, err := WebhookMiddleware(next, WebhookMiddlewareConfig{
		Secret:             "s3cr3t",
		AuthMode:           "header",
		HeaderName:         "X-Webhook-Secret",
		RejectQuerySecrets: true,
	})
	if err != nil {
		t.Fatalf("middleware init error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/hook?token=abc", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "s3cr3t")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
	if calls != 0 {
		t.Fatalf("handler should not be called, got %d", calls)
	}
}

func TestWebhookMiddlewareIdempotencySkipsDuplicates(t *testing.T) {
	t.Parallel()

	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	h, err := WebhookMiddleware(next, WebhookMiddlewareConfig{
		Secret:             "token",
		AuthMode:           "header",
		HeaderName:         "X-Webhook-Secret",
		IdempotencyEnabled: true,
		IdempotencyTTLSec:  60,
	})
	if err != nil {
		t.Fatalf("middleware init error: %v", err)
	}

	body := `{"update_id":123}`
	req1 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	req1.Header.Set("X-Webhook-Secret", "token")
	res1 := httptest.NewRecorder()
	h.ServeHTTP(res1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	req2.Header.Set("X-Webhook-Secret", "token")
	res2 := httptest.NewRecorder()
	h.ServeHTTP(res2, req2)

	if res1.Code != http.StatusOK || res2.Code != http.StatusOK {
		t.Fatalf("expected both requests to return 200, got %d and %d", res1.Code, res2.Code)
	}
	if calls != 1 {
		t.Fatalf("expected one handler call due to dedupe, got %d", calls)
	}
}

func TestWebhookMiddlewareRateLimit(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h, err := WebhookMiddleware(next, WebhookMiddlewareConfig{
		Secret:             "token",
		AuthMode:           "header",
		HeaderName:         "X-Webhook-Secret",
		IdempotencyEnabled: false,
		RateLimitPerMinute: 1,
	})
	if err != nil {
		t.Fatalf("middleware init error: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"request_id":"a"}`))
	req1.Header.Set("X-Webhook-Secret", "token")
	res1 := httptest.NewRecorder()
	h.ServeHTTP(res1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"request_id":"b"}`))
	req2.Header.Set("X-Webhook-Secret", "token")
	res2 := httptest.NewRecorder()
	h.ServeHTTP(res2, req2)

	if res1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", res1.Code)
	}
	if res2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", res2.Code)
	}
}

func TestWebhookMiddlewarePathAuth(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h, err := WebhookMiddleware(next, WebhookMiddlewareConfig{
		Secret:   "abc123",
		AuthMode: "path",
	})
	if err != nil {
		t.Fatalf("middleware init error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/hook/abc123", strings.NewReader("{}"))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}
