package cloudbuild

import "github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"

var (
	ErrBuildSubmit = apperr.E("C-BUILD-001", "Failed to submit Cloud Build")
	ErrBuildPoll   = apperr.E("C-BUILD-002", "Failed to poll Cloud Build")
)