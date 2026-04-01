package safety

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebhookMiddlewareConfig struct {
	Secret              string
	AuthMode            string // header|path
	HeaderName          string
	RejectQuerySecrets  bool
	IdempotencyEnabled  bool
	IdempotencyTTLSec   int
	RateLimitPerMinute  int
	SuspiciousBurstHits int
	Logger              *slog.Logger
}

func (c WebhookMiddlewareConfig) Validate() error {
	if c.RejectQuerySecrets {
		// Explicit policy: query-string secrets are insecure and unsupported.
	}
	mode := strings.ToLower(strings.TrimSpace(c.AuthMode))
	if mode == "" {
		mode = "header"
	}
	if mode != "header" && mode != "path" {
		return ErrInvalidWebhookAuthMode
	}
	if strings.TrimSpace(c.Secret) == "" {
		return ErrMissingWebhookSecret
	}
	if mode == "header" && strings.TrimSpace(c.HeaderName) == "" {
		return ErrMissingWebhookHeader
	}
	return nil
}

type middlewareError string

func (e middlewareError) Error() string { return string(e) }

var (
	ErrMissingWebhookSecret   = middlewareError("webhook auth requires a non-empty secret")
	ErrMissingWebhookHeader   = middlewareError("webhook header auth requires a header name")
	ErrInvalidWebhookAuthMode = middlewareError("webhook auth mode must be 'header' or 'path'")
)

type memoryIdempotencyStore struct {
	mu   sync.Mutex
	data map[string]time.Time
}

func newMemoryIdempotencyStore() *memoryIdempotencyStore {
	return &memoryIdempotencyStore{data: map[string]time.Time{}}
}

func (s *memoryIdempotencyStore) seen(key string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, exp := range s.data {
		if now.After(exp) {
			delete(s.data, k)
		}
	}
	exp, ok := s.data[key]
	return ok && now.Before(exp)
}

func (s *memoryIdempotencyStore) mark(key string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	s.mu.Lock()
	s.data[key] = time.Now().Add(ttl)
	s.mu.Unlock()
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]windowCounter
}

type windowCounter struct {
	windowStart time.Time
	hits        int
}

func newFixedWindowRateLimiter(limit int) *fixedWindowRateLimiter {
	if limit <= 0 {
		limit = 120
	}
	return &fixedWindowRateLimiter{
		limit:   limit,
		windows: map[string]windowCounter{},
	}
}

func (r *fixedWindowRateLimiter) allow(key string) (bool, int) {
	now := time.Now().UTC()
	window := now.Truncate(time.Minute)

	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.windows[key]
	if st.windowStart.IsZero() || !st.windowStart.Equal(window) {
		st = windowCounter{windowStart: window, hits: 0}
	}
	st.hits++
	r.windows[key] = st
	return st.hits <= r.limit, st.hits
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func WebhookMiddleware(next http.Handler, cfg WebhookMiddlewareConfig) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	if mode == "" {
		mode = "header"
	}
	headerName := strings.TrimSpace(cfg.HeaderName)
	if headerName == "" {
		headerName = "X-Webhook-Secret"
	}
	idemTTL := time.Duration(cfg.IdempotencyTTLSec) * time.Second
	if idemTTL <= 0 {
		idemTTL = time.Hour
	}
	suspiciousThreshold := cfg.SuspiciousBurstHits
	if suspiciousThreshold <= 0 {
		suspiciousThreshold = 5
	}

	idemStore := newMemoryIdempotencyStore()
	rateLimiter := newFixedWindowRateLimiter(cfg.RateLimitPerMinute)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		source := classifySource(r)
		requestType := strings.ToLower(strings.TrimSpace(r.Method))
		path := strings.TrimSpace(r.URL.Path)

		if cfg.RejectQuerySecrets && hasSensitiveQueryParam(r.URL.Query()) {
			http.Error(rec, "query-string secrets are not allowed", http.StatusBadRequest)
			logWebhook(logger, rec.status, start, path, source, requestType, "query_secret_rejected")
			return
		}

		authOK := false
		switch mode {
		case "header":
			authOK = strings.TrimSpace(r.Header.Get(headerName)) == cfg.Secret
		case "path":
			authOK = strings.Contains(path, "/"+cfg.Secret) || strings.HasSuffix(path, "/"+cfg.Secret)
		}
		if !authOK {
			http.Error(rec, "unauthorized webhook request", http.StatusUnauthorized)
			logWebhook(logger, rec.status, start, path, source, requestType, "auth_failed")
			return
		}

		clientKey := clientIdentifier(r) + "|" + path
		allowed, hits := rateLimiter.allow(clientKey)
		if !allowed {
			http.Error(rec, "rate limit exceeded", http.StatusTooManyRequests)
			event := "rate_limited"
			if hits >= cfg.RateLimitPerMinute+suspiciousThreshold {
				event = "suspicious_burst"
			}
			logWebhook(logger, rec.status, start, path, source, requestType, event)
			return
		}

		if cfg.IdempotencyEnabled {
			idKey, body := deriveIdempotencyKey(r)
			if len(body) > 0 {
				r.Body = io.NopCloser(bytes.NewReader(body))
				r.ContentLength = int64(len(body))
			}
			if idKey != "" {
				if idemStore.seen(idKey) {
					rec.WriteHeader(http.StatusOK)
					logWebhook(logger, rec.status, start, path, source, requestType, "duplicate_ignored")
					return
				}
				idemStore.mark(idKey, idemTTL)
			}
		}

		next.ServeHTTP(rec, r)
		logWebhook(logger, rec.status, start, path, source, requestType, "ok")
	}), nil
}

func deriveIdempotencyKey(r *http.Request) (string, []byte) {
	if v := strings.TrimSpace(r.Header.Get("X-Request-Id")); v != "" {
		return "req:" + v, nil
	}
	if v := strings.TrimSpace(r.Header.Get("Idempotency-Key")); v != "" {
		return "idem:" + v, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return "", body
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", body
	}
	if v, ok := m["update_id"]; ok {
		return "telegram:" + stringify(v), body
	}
	if v, ok := m["request_id"]; ok {
		return "request:" + stringify(v), body
	}
	return "", body
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(formatFloat(t)), ".0"), ".")
	default:
		return ""
	}
}

func formatFloat(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func hasSensitiveQueryParam(q map[string][]string) bool {
	for k := range q {
		key := strings.ToLower(strings.TrimSpace(k))
		if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "key") {
			return true
		}
	}
	return false
}

func classifySource(r *http.Request) string {
	ua := strings.ToLower(strings.TrimSpace(r.UserAgent()))
	switch {
	case strings.Contains(ua, "telegram"):
		return "telegram"
	case strings.Contains(ua, "google"):
		return "gcp"
	case strings.Contains(ua, "curl"):
		return "cli"
	default:
		return "unknown"
	}
}

func clientIdentifier(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func logWebhook(logger *slog.Logger, status int, start time.Time, path, source, requestType, event string) {
	latencyMs := time.Since(start).Milliseconds()
	logger.Info("webhook_request",
		"path", path,
		"source_classification", source,
		"status", status,
		"latency_ms", latencyMs,
		"request_type", requestType,
		"event", event,
	)
}
