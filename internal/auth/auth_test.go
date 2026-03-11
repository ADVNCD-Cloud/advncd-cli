package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

func TestGetAccessToken_UsesExistingAppTokenWithoutRefresh(t *testing.T) {
	var refreshCalls int32
	var gcpCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/token/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			http.Error(w, "must not be called", http.StatusBadRequest)
		case "/api/auth/gcp/access-token":
			atomic.AddInt32(&gcpCalls, 1)
			authz := r.Header.Get("Authorization")
			if authz != "Bearer app-old" {
				http.Error(w, "bad bearer", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gcp-token",
				"token_type":   "Bearer",
				"expires_in":   3599,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("ADVNCD_AUTH_BASE_URL", srv.URL)

	store, err := creds.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore: %v", err)
	}
	err = store.Save(creds.Credentials{
		Version:              2,
		Email:                "user@example.com",
		AuthBaseURL:          srv.URL,
		AppAccessToken:       "app-old",
		AppRefreshToken:      "refresh-old",
		AppAccessTokenExpiry: time.Now().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	tb, err := GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if tb.AccessToken != "gcp-token" {
		t.Fatalf("unexpected gcp token: %q", tb.AccessToken)
	}
	if atomic.LoadInt32(&refreshCalls) != 0 {
		t.Fatalf("refresh called unexpectedly")
	}
	if atomic.LoadInt32(&gcpCalls) != 1 {
		t.Fatalf("expected exactly one gcp call")
	}
}

func TestGetAccessToken_RefreshesExpiredAppToken(t *testing.T) {
	var refreshCalls int32
	var gcpCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/token/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			if strings.TrimSpace(in["app_refresh_token"]) != "refresh-old" {
				http.Error(w, "bad refresh token", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"app_access_token": "app-new",
				"expires_in":       900,
			})
		case "/api/auth/gcp/access-token":
			atomic.AddInt32(&gcpCalls, 1)
			authz := r.Header.Get("Authorization")
			if authz != "Bearer app-new" {
				http.Error(w, "bad bearer", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gcp-token-new",
				"token_type":   "Bearer",
				"expires_in":   3599,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("ADVNCD_AUTH_BASE_URL", srv.URL)

	store, err := creds.DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore: %v", err)
	}
	err = store.Save(creds.Credentials{
		Version:              2,
		Email:                "user@example.com",
		AuthBaseURL:          srv.URL,
		AppAccessToken:       "app-old",
		AppRefreshToken:      "refresh-old",
		AppAccessTokenExpiry: time.Now().Add(-1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	tb, err := GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if tb.AccessToken != "gcp-token-new" {
		t.Fatalf("unexpected gcp token: %q", tb.AccessToken)
	}
	if atomic.LoadInt32(&refreshCalls) != 1 {
		t.Fatalf("expected one refresh call")
	}
	if atomic.LoadInt32(&gcpCalls) != 1 {
		t.Fatalf("expected one gcp call")
	}

	updated, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if updated.AppAccessToken != "app-new" {
		t.Fatalf("expected saved app token to be refreshed, got %q", updated.AppAccessToken)
	}
	if time.Until(updated.AppAccessTokenExpiry) <= 0 {
		t.Fatalf("expected refreshed app access expiry in future")
	}
}
