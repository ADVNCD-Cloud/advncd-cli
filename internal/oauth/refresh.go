package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

func RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, apperr.New(apperr.AuthMissingClientID)
	}
	if strings.TrimSpace(refreshToken) == "" {
		return nil, apperr.New(apperr.AuthTokenExchange).
			WithFix("No refresh_token found. Run 'advncd login' again.")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	if strings.TrimSpace(clientSecret) != "" {
		form.Set("client_secret", clientSecret)
	}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, apperr.New(apperr.AuthTokenExchange).WithCause(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(apperr.AuthTokenExchange).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var te tokenError
		_ = json.Unmarshal(body, &te)

		ae := apperr.New(apperr.AuthTokenExchange).
			WithMeta("http_status", res.Status).
			WithMeta("oauth_error", te.Error).
			WithMeta("oauth_error_description", te.ErrorDescription)

		if te.Error == "" {
			ae = ae.WithMeta("raw_body", string(body))
		}

		ae = ae.WithFix("Try 'advncd login' again to refresh consent and tokens.")
		return nil, ae
	}

	var out TokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, apperr.New(apperr.AuthTokenExchange).WithCause(err).
			WithMeta("raw_body", string(body))
	}

	if out.AccessToken == "" {
		return nil, apperr.New(apperr.AuthTokenExchange).
			WithMeta("raw_body", string(body)).
			WithFix("Token endpoint returned no access_token.")
	}

	// Note: refresh_token is usually NOT returned here; keep old one.
	return &out, nil
}