package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

const userInfoEndpoint = "https://openidconnect.googleapis.com/v1/userinfo"

type UserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func FetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return nil, apperr.New(apperr.AuthUserInfo).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(apperr.AuthUserInfo).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(apperr.AuthUserInfo).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(body)).
			WithFix("Ensure scopes include 'openid email profile'.")
	}

	var ui UserInfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return nil, apperr.New(apperr.AuthUserInfo).WithCause(err).
			WithMeta("raw_body", string(body))
	}
	return &ui, nil
}