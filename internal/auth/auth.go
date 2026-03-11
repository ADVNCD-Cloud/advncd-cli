package auth

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/authbroker"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

var (
	ErrNotLoggedIn = apperr.E("A-AUTH-402", "Not logged in")
)

// TokenBundle is what most commands need.
type TokenBundle struct {
	AccessToken string
	Expiry      time.Time
	Email       string
	CredsPath   string
	AppExpiry   time.Time
	AuthBaseURL string
}

type SessionInfo struct {
	Email       string
	AppExpiry   time.Time
	CredsPath   string
	AuthBaseURL string
}

func ResolveAuthBaseURL(stored string) string {
	return firstNonEmpty(
		os.Getenv("ADVNCD_AUTH_BASE_URL"),
		stored,
		authbroker.DefaultBaseURL,
	)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func GetSessionInfo() (*SessionInfo, error) {
	store, err := creds.DefaultStore()
	if err != nil {
		return nil, err
	}

	c, err := store.Load()
	if err != nil {
		return nil, err
	}
	if c == nil || strings.TrimSpace(c.AppRefreshToken) == "" {
		return nil, apperr.New(ErrNotLoggedIn).
			WithFix("Run: advncd login")
	}

	return &SessionInfo{
		Email:       c.Email,
		AppExpiry:   c.AppAccessTokenExpiry,
		CredsPath:   store.Path,
		AuthBaseURL: ResolveAuthBaseURL(c.AuthBaseURL),
	}, nil
}

// GetAccessToken loads local app creds, refreshes if needed, then asks auth broker for a GCP access token.
func GetAccessToken(ctx context.Context) (*TokenBundle, error) {
	store, err := creds.DefaultStore()
	if err != nil {
		return nil, err
	}

	c, err := store.Load()
	if err != nil {
		return nil, err
	}
	if c == nil || strings.TrimSpace(c.AppRefreshToken) == "" {
		return nil, apperr.New(ErrNotLoggedIn).
			WithFix("Run: advncd login")
	}

	baseURL := ResolveAuthBaseURL(c.AuthBaseURL)
	broker := authbroker.New(baseURL)

	// Refresh app token if expiring soon (skew 30s)
	if time.Until(c.AppAccessTokenExpiry) < 30*time.Second {
		tok, err := broker.Refresh(ctx, c.AppRefreshToken)
		if err != nil {
			return nil, err
		}

		c.AppAccessToken = tok.AppAccessToken
		c.AppAccessTokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
		c.AuthBaseURL = baseURL

		if err := store.Save(*c); err != nil {
			return nil, err
		}
	}

	gcpTok, err := broker.GCPAccessToken(ctx, c.AppAccessToken)
	if err != nil {
		return nil, err
	}
	gcpExpiry := time.Now().Add(time.Duration(gcpTok.ExpiresIn) * time.Second)

	return &TokenBundle{
		AccessToken: gcpTok.AccessToken,
		Expiry:      gcpExpiry,
		Email:       c.Email,
		CredsPath:   store.Path,
		AppExpiry:   c.AppAccessTokenExpiry,
		AuthBaseURL: baseURL,
	}, nil
}
