package gcpbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrBudgetCreate         = apperr.E("B-BUDGET-001", "Failed to create billing budget")
	ErrProjectBillingLookup = apperr.E("B-BUDGET-002", "Failed to read project billing info")
)

type ProjectBillingInfo struct {
	ProjectID          string `json:"projectId"`
	BillingAccountName string `json:"billingAccountName"`
	BillingEnabled     bool   `json:"billingEnabled"`
}

func GetProjectBillingInfo(ctx context.Context, accessToken, projectID string) (*ProjectBillingInfo, error) {
	projectID = strings.TrimSpace(strings.TrimPrefix(projectID, "projects/"))
	u := "https://cloudbilling.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/billingInfo"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, apperr.New(ErrProjectBillingLookup).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(ErrProjectBillingLookup).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(ErrProjectBillingLookup).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithMeta("project_id", projectID)
	}

	var out ProjectBillingInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, apperr.New(ErrProjectBillingLookup).WithCause(err).
			WithMeta("raw_body", string(raw))
	}
	return &out, nil
}

type BudgetCreateInput struct {
	BillingAccountName string
	ProjectNumber      string
	DisplayName        string
	AmountEUR          float64
	Thresholds         []float64
}

func CreateProjectBudget(ctx context.Context, accessToken string, in BudgetCreateInput) error {
	account := normalizeBillingAccountName(in.BillingAccountName)
	parent := "https://billingbudgets.googleapis.com/v1beta1/" + account + "/budgets"

	amountUnits, amountNanos := splitAmount(in.AmountEUR)
	thresholds := in.Thresholds
	if len(thresholds) == 0 {
		thresholds = []float64{0.5, 0.9, 1.0}
	}

	type thresholdRule struct {
		ThresholdPercent float64 `json:"thresholdPercent"`
	}
	rules := make([]thresholdRule, 0, len(thresholds))
	for _, t := range thresholds {
		if t <= 0 {
			continue
		}
		rules = append(rules, thresholdRule{ThresholdPercent: t})
	}

	if strings.TrimSpace(in.DisplayName) == "" {
		in.DisplayName = "advncd-safety-budget-" + strings.TrimSpace(in.ProjectNumber)
	}

	payload := map[string]any{
		"displayName": in.DisplayName,
		"budgetFilter": map[string]any{
			"projects": []string{"projects/" + strings.TrimSpace(in.ProjectNumber)},
		},
		"amount": map[string]any{
			"specifiedAmount": map[string]any{
				"currencyCode": "EUR",
				"units":        strconv.FormatInt(amountUnits, 10),
				"nanos":        amountNanos,
			},
		},
		"thresholdRules": rules,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parent, bytes.NewReader(body))
	if err != nil {
		return apperr.New(ErrBudgetCreate).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrBudgetCreate).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrBudgetCreate).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithMeta("billing_account", account)
	}
	return nil
}

func splitAmount(v float64) (int64, int32) {
	if v < 0 {
		v = 0
	}
	units := int64(v)
	nanos := int32(math.Round((v - float64(units)) * 1_000_000_000))
	if nanos < 0 {
		nanos = 0
	}
	return units, nanos
}
