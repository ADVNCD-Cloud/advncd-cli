package gcpbilling

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

var (
	ErrBillingAccountsList = apperr.E("B-BILL-001", "Failed to list billing accounts")
	ErrProjectBillingLink  = apperr.E("B-BILL-002", "Failed to link project billing account")
)

type BillingAccount struct {
	Name        string `json:"name"` // billingAccounts/XXXXXX-XXXXXX-XXXXXX
	DisplayName string `json:"displayName"`
	Open        bool   `json:"open"`
}

type billingAccountsResp struct {
	BillingAccounts []BillingAccount `json:"billingAccounts"`
	NextPageToken   string           `json:"nextPageToken"`
}

func ListOpenBillingAccounts(ctx context.Context, accessToken string) ([]BillingAccount, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	pageToken := ""
	all := make([]BillingAccount, 0, 16)

	for {
		u, _ := url.Parse("https://cloudbilling.googleapis.com/v1/billingAccounts")
		q := u.Query()
		q.Set("pageSize", "100")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, apperr.New(ErrBillingAccountsList).WithCause(err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := client.Do(req)
		if err != nil {
			return nil, apperr.New(ErrBillingAccountsList).WithCause(err)
		}

		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, apperr.New(ErrBillingAccountsList).
				WithMeta("http_status", res.Status).
				WithMeta("raw_body", string(body)).
				WithFix("Ensure you have permission to view billing accounts.")
		}

		var out billingAccountsResp
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, apperr.New(ErrBillingAccountsList).WithCause(err).
				WithMeta("raw_body", string(body))
		}

		for _, acc := range out.BillingAccounts {
			if !acc.Open {
				continue
			}
			if strings.TrimSpace(acc.Name) == "" {
				continue
			}
			all = append(all, acc)
		}

		if strings.TrimSpace(out.NextPageToken) == "" {
			break
		}
		pageToken = out.NextPageToken
	}

	return all, nil
}

func LinkProjectBilling(ctx context.Context, accessToken, projectID, billingAccountName string) error {
	projectID = strings.TrimSpace(strings.TrimPrefix(projectID, "projects/"))
	billingAccountName = normalizeBillingAccountName(billingAccountName)

	body, _ := json.Marshal(map[string]interface{}{
		"billingAccountName": billingAccountName,
		"billingEnabled":     true,
	})

	u := "https://cloudbilling.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/billingInfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return apperr.New(ErrProjectBillingLink).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrProjectBillingLink).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrProjectBillingLink).
			WithMeta("http_status", res.Status).
			WithMeta("project_id", projectID).
			WithMeta("billing_account", billingAccountName).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure your account has permission to link billing to this project.")
	}

	return nil
}

func normalizeBillingAccountName(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "billingAccounts/") {
		return v
	}
	return "billingAccounts/" + v
}
