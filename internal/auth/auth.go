package auth

import (
	"context"
	"os"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/oauth"
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
}

// GetAccessToken loads local creds, refreshes if needed, and returns a valid access token.
func GetAccessToken(ctx context.Context) (*TokenBundle, error) {
	store, err := creds.DefaultStore()
	if err != nil {
		return nil, err
	}

	c, err := store.Load()
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperr.New(ErrNotLoggedIn).
			WithFix("Run: advncd login")
	}

	clientSecret := os.Getenv("ADVNCD_GCP_CLIENT_SECRET")

	// Refresh if expiring soon (skew 30s)
	if time.Until(c.Expiry) < 30*time.Second {
		tok, err := oauth.RefreshAccessToken(ctx, c.ClientID, clientSecret, c.RefreshToken)
		if err != nil {
			return nil, err
		}

		c.AccessToken = tok.AccessToken
		c.TokenType = tok.TokenType
		c.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

		if err := store.Save(*c); err != nil {
			return nil, err
		}
	}

	return &TokenBundle{
		AccessToken: c.AccessToken,
		Expiry:      c.Expiry,
		Email:       c.Email,
		CredsPath:   store.Path,
	}, nil
}

// GetIdentity verifies the token by calling userinfo.
func GetIdentity(ctx context.Context) (*oauth.UserInfo, *TokenBundle, error) {
	tb, err := GetAccessToken(ctx)
	if err != nil {
		return nil, nil, err
	}
	me, err := oauth.FetchUserInfo(ctx, tb.AccessToken)
	if err != nil {
		return nil, nil, err
	}
	return me, tb, nil
}