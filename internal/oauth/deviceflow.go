package oauth

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

const (
	deviceEndpoint = "https://oauth2.googleapis.com/device/code"
)

type DeviceFlowRequest struct {
	ClientID string
	Scopes   []string
}

type DeviceFlowResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"` // Google returns this field
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type googleErr struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func StartDeviceFlow(ctx context.Context, req DeviceFlowRequest) (*DeviceFlowResponse, error) {
	if strings.TrimSpace(req.ClientID) == "" {
		return nil, apperr.New(apperr.AuthMissingClientID)
	}
	if len(req.Scopes) == 0 {
		return nil, apperr.New(apperr.AuthInvalidScopes).
			WithFix("Provide at least one OAuth scope.")
	}

	form := url.Values{}
	form.Set("client_id", req.ClientID)
	form.Set("scope", strings.Join(req.Scopes, " "))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceEndpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, apperr.New(apperr.AuthHTTPBuild).WithCause(err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, apperr.New(apperr.AuthHTTPDo).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var ge googleErr
		_ = json.Unmarshal(body, &ge)

		ae := apperr.New(apperr.AuthDeviceFlowFailed).
			WithMeta("http_status", res.Status).
			WithMeta("google_error", ge.Error).
			WithMeta("google_error_description", ge.ErrorDescription).
			WithFix("Verify your OAuth client configuration (this flow may not support GCP scopes).").
			WithFix("Advncd uses Authorization Code + PKCE for GCP; prefer that flow.")

		// Attach raw body if not JSON or empty
		if ge.Error == "" {
			ae = ae.WithMeta("raw_body", string(body))
		}
		return nil, ae
	}

	var out DeviceFlowResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, apperr.New(apperr.AuthJSONDecode).WithCause(err).
			WithMeta("raw_body", string(body))
	}
	if out.DeviceCode == "" || out.UserCode == "" || out.VerificationURL == "" {
		return nil, apperr.New(apperr.AuthDeviceFlowMalformed).
			WithMeta("raw_body", string(body)).
			WithFix("Google returned an unexpected response; try again or check your OAuth client configuration.")
	}

	// Google sometimes omits interval; spec default 5
	if out.Interval == 0 {
		out.Interval = 5
	}

	return &out, nil
}