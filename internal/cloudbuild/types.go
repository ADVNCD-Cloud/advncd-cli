package cloudbuild

import "time"

type Build struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	LogURL string `json:"logUrl"`
}

type SubmitRequest struct {
	AccessToken string
	ProjectID   string
	SourceDir   string
	Image       string
}

type WaitRequest struct {
	AccessToken string
	ProjectID   string
	BuildID     string
	PollEvery time.Duration // seconds (we'll store duration as seconds to avoid extra imports here)
}