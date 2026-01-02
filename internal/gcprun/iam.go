package gcprun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrRunIAM = apperr.E("C-RUN-004", "Failed to configure Cloud Run IAM")

type iamPolicy struct {
	Version  int `json:"version,omitempty"`
	Bindings []struct {
		Role    string   `json:"role"`
		Members []string `json:"members"`
	} `json:"bindings,omitempty"`
	Etag string `json:"etag,omitempty"`
}

func AllowUnauthenticated(ctx context.Context, accessToken, projectID, region, serviceName string) error {
	base := fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/services/%s", projectID, region, serviceName)

	// 1) get policy
	getURL := base + ":getIamPolicy"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return apperr.New(ErrRunIAM).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrRunIAM).WithCause(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrRunIAM).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure you have permission to set IAM policy on Cloud Run service.")
	}

	var pol iamPolicy
	if err := json.Unmarshal(raw, &pol); err != nil {
		return apperr.New(ErrRunIAM).WithCause(err).
			WithMeta("raw_body", string(raw))
	}

	// 2) ensure binding exists
	const role = "roles/run.invoker"
	const member = "allUsers"

	found := false
	for i := range pol.Bindings {
		if pol.Bindings[i].Role == role {
			// ensure member
			for _, m := range pol.Bindings[i].Members {
				if m == member {
					found = true
					break
				}
			}
			if !found {
				pol.Bindings[i].Members = append(pol.Bindings[i].Members, member)
				found = true
			}
			break
		}
	}

	if !found {
		pol.Bindings = append(pol.Bindings, struct {
			Role    string   `json:"role"`
			Members []string `json:"members"`
		}{Role: role, Members: []string{member}})
	}

	// 3) set policy (must include etag)
	setURL := base + ":setIamPolicy"
	payload := map[string]any{
		"policy": pol,
	}
	b, _ := json.Marshal(payload)

	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, setURL, bytes.NewReader(b))
	if err != nil {
		return apperr.New(ErrRunIAM).WithCause(err)
	}
	req2.Header.Set("Authorization", "Bearer "+accessToken)
	req2.Header.Set("Content-Type", "application/json; charset=utf-8")

	res2, err := client.Do(req2)
	if err != nil {
		return apperr.New(ErrRunIAM).WithCause(err)
	}
	raw2, _ := io.ReadAll(res2.Body)
	_ = res2.Body.Close()

	if res2.StatusCode < 200 || res2.StatusCode >= 300 {
		return apperr.New(ErrRunIAM).
			WithMeta("http_status", res2.Status).
			WithMeta("raw_body", string(raw2)).
			WithFix("If your org forbids public access, use authenticated access instead.")
	}

	return nil
}