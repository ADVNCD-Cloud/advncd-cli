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

const tokenEndpoint = "https://oauth2.googleapis.com/token"

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

type tokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func ExchangeAuthCode(ctx context.Context, clientID, clientSecret, code, redirectURI, codeVerifier string) (*TokenResponse, error) {
	if strings.TrimSpace(clientID) == "" {
		return nil, apperr.New(apperr.AuthMissingClientID)
	}
	if strings.TrimSpace(code) == "" {
		return nil, apperr.New(apperr.AuthMissingCode)
	}
	if strings.TrimSpace(redirectURI) == "" || strings.TrimSpace(codeVerifier) == "" {
		return nil, apperr.New(apperr.AuthTokenExchange).
			WithFix("Internal error: missing redirect_uri or code_verifier.")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	if strings.TrimSpace(clientSecret) != "" {
		form.Set("client_secret", clientSecret)
	}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)

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

		ae = ae.WithFix("Ensure your OAuth client is type 'Desktop' (installed app).").
			WithFix("If Google requires a secret for this client, export ADVNCD_GCP_CLIENT_SECRET and retry.")

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

	return &out, nil
}