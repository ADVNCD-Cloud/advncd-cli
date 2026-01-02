package gcpcrm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrProjectsList = apperr.E("B-CRM-001", "Failed to list GCP projects")
)

type Project struct {
	ProjectID      string `json:"projectId"`
	Name           string `json:"name"`
	LifecycleState string `json:"lifecycleState"`
}

type listResp struct {
	Projects      []Project `json:"projects"`
	NextPageToken string    `json:"nextPageToken"`
}

func ListProjects(ctx context.Context, accessToken string) ([]Project, error) {
	var all []Project
	pageToken := ""

	client := &http.Client{Timeout: 20 * time.Second}

	for {
		u, _ := url.Parse("https://cloudresourcemanager.googleapis.com/v1/projects")
		q := u.Query()
		// You can adjust pageSize, default is fine too
		q.Set("pageSize", "200")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, apperr.New(ErrProjectsList).WithCause(err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := client.Do(req)
		if err != nil {
			return nil, apperr.New(ErrProjectsList).WithCause(err).
				WithFix("Check your internet connection and try again.")
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, apperr.New(ErrProjectsList).
				WithMeta("http_status", res.Status).
				WithMeta("raw_body", string(body)).
				WithFix("Ensure you are logged in: advncd login").
				WithFix("Ensure your account has permission to list projects.")
		}

		var out listResp
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, apperr.New(ErrProjectsList).WithCause(err).
				WithMeta("raw_body", string(body))
		}

		for _, p := range out.Projects {
			// keep ACTIVE only
			if p.LifecycleState == "ACTIVE" {
				all = append(all, p)
			}
		}

		if out.NextPageToken == "" {
			break
		}
		pageToken = out.NextPageToken
	}

	return all, nil
}