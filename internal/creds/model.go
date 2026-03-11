package creds

import "time"

type Credentials struct {
	Version int `json:"version"`

	Email       string `json:"email"`
	AuthBaseURL string `json:"auth_base_url,omitempty"`

	AppAccessToken      string    `json:"app_access_token,omitempty"`
	AppRefreshToken     string    `json:"app_refresh_token,omitempty"`
	AppAccessTokenExpiry time.Time `json:"access_token_expires_at,omitempty"`

	// Legacy fields kept for backward-compatible decoding of older credentials files.
	Scopes       []string  `json:"scopes,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
}
