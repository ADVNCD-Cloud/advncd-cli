package authbroker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

const DefaultBaseURL = "https://www.andreitazetdinov.com"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type StartResponse struct {
	SessionID       string `json:"session_id"`
	VerifyURL       string `json:"verify_url"`
	ExpiresIn       int    `json:"expires_in"`
	IntervalSeconds int    `json:"interval_seconds"`
}

type PollResponse struct {
	Status            string `json:"status"`
	AppAccessToken    string `json:"app_access_token,omitempty"`
	AppRefreshToken   string `json:"app_refresh_token,omitempty"`
	ExpiresIn         int    `json:"expires_in,omitempty"`
	Email             string `json:"email,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}

type RefreshResponse struct {
	AppAccessToken string `json:"app_access_token"`
	ExpiresIn      int    `json:"expires_in"`
}

type GCPAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type LogoutResponse struct {
	Success bool `json:"success"`
}

func (c *Client) Start(ctx context.Context, deviceName string) (*StartResponse, error) {
	body := map[string]string{}
	if strings.TrimSpace(deviceName) != "" {
		body["device_name"] = strings.TrimSpace(deviceName)
	}

	var out StartResponse
	if err := c.postJSON(ctx, "/api/auth/cli/start", body, "", &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.SessionID) == "" || strings.TrimSpace(out.VerifyURL) == "" {
		return nil, apperr.New(apperr.AuthBrokerProtocol).
			WithFix("Auth broker returned malformed /cli/start response.")
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 600
	}
	if out.IntervalSeconds <= 0 {
		out.IntervalSeconds = 5
	}
	return &out, nil
}

func (c *Client) Poll(ctx context.Context, sessionID string) (*PollResponse, error) {
	u := c.baseURL + "/api/auth/cli/poll?session_id=" + url.QueryEscape(strings.TrimSpace(sessionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, apperr.New(apperr.AuthBrokerProtocol).WithCause(err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.New(apperr.AuthBrokerUnavailable).WithCause(err).
			WithFix("Check ADVNCD_AUTH_BASE_URL and network access.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode == http.StatusGone {
		return &PollResponse{Status: "expired"}, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		var out PollResponse
		_ = json.Unmarshal(body, &out)
		if out.RetryAfterSeconds <= 0 {
			out.RetryAfterSeconds = 3
		}
		return &out, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(apperr.AuthBrokerUnavailable).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(body))
	}

	var out PollResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, apperr.New(apperr.AuthBrokerProtocol).WithCause(err).
			WithMeta("raw_body", string(body))
	}
	return &out, nil
}

func (c *Client) Refresh(ctx context.Context, appRefreshToken string) (*RefreshResponse, error) {
	in := map[string]string{"app_refresh_token": strings.TrimSpace(appRefreshToken)}
	var out RefreshResponse
	if err := c.postJSON(ctx, "/api/auth/token/refresh", in, "", &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AppAccessToken) == "" {
		return nil, apperr.New(apperr.AuthBrokerProtocol).
			WithFix("Auth broker returned malformed /token/refresh response.")
	}
	return &out, nil
}

func (c *Client) GCPAccessToken(ctx context.Context, appAccessToken string) (*GCPAccessTokenResponse, error) {
	var out GCPAccessTokenResponse
	if err := c.postJSON(ctx, "/api/auth/gcp/access-token", map[string]string{}, strings.TrimSpace(appAccessToken), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, apperr.New(apperr.AuthBrokerProtocol).
			WithFix("Auth broker returned malformed /gcp/access-token response.")
	}
	return &out, nil
}

func (c *Client) Logout(ctx context.Context, appRefreshToken string) error {
	in := map[string]string{"app_refresh_token": strings.TrimSpace(appRefreshToken)}
	var out LogoutResponse
	if err := c.postJSON(ctx, "/api/auth/logout", in, "", &out); err != nil {
		return err
	}
	if !out.Success {
		return apperr.New(apperr.AuthBrokerProtocol).
			WithFix("Auth broker returned unexpected /logout response.")
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, in any, bearerToken string, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return apperr.New(apperr.AuthBrokerProtocol).WithCause(err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return apperr.New(apperr.AuthBrokerProtocol).WithCause(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return apperr.New(apperr.AuthBrokerUnavailable).WithCause(err).
			WithFix("Check ADVNCD_AUTH_BASE_URL and network access.")
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		entry := apperr.AuthBrokerUnavailable
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			entry = apperr.AuthInvalidRefresh
		}
		ae := apperr.New(entry).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(respBody))
		if entry.Code == apperr.AuthInvalidRefresh.Code {
			ae = ae.WithFix("Run 'advncd login' to re-authenticate.")
		}
		return ae
	}

	if out == nil {
		return nil
	}
	if len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return apperr.New(apperr.AuthBrokerProtocol).WithCause(err).
			WithMeta("raw_body", string(respBody))
	}
	return nil
}
