package creds

import "time"

type Credentials struct {
	Version int `json:"version"`

	Email  string   `json:"email"`
	Scopes []string `json:"scopes"`

	ClientID string `json:"client_id"`

	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
}