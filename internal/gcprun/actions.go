package gcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrRunDelete   = apperr.E("C-RUN-004", "Failed to delete Cloud Run service")
	ErrRunRedeploy = apperr.E("C-RUN-005", "Failed to redeploy Cloud Run service")
)

func DeleteService(ctx context.Context, accessToken, projectID, region, serviceName string) error {
	req := DeployRequest{
		AccessToken: accessToken,
		ProjectID:   projectID,
		Region:      region,
		ServiceName: serviceName,
	}
	u := serviceURL(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return apperr.New(ErrRunDelete).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return apperr.New(ErrRunDelete).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == 404 {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrRunDelete).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw))
	}

	var op opLike
	if err := json.Unmarshal(raw, &op); err == nil && op.Name != "" {
		return waitOperation(ctx, accessToken, op.Name)
	}
	return nil
}

func RedeployService(ctx context.Context, accessToken, projectID, region, serviceName string) error {
	req := DeployRequest{
		AccessToken: accessToken,
		ProjectID:   projectID,
		Region:      region,
		ServiceName: serviceName,
	}
	exists, current, err := getService(ctx, req)
	if err != nil {
		return apperr.New(ErrRunRedeploy).WithCause(err)
	}
	if !exists || current == nil {
		return apperr.New(ErrRunRedeploy).
			WithMeta("service", serviceName).
			WithFix("Service not found for redeploy.")
	}

	if current.Template.Labels == nil {
		current.Template.Labels = map[string]string{}
	}
	current.Template.Labels["advncd-redeploy-at"] = fmt.Sprintf("%d", time.Now().UTC().UnixNano())

	opName, err := patchService(ctx, req, current, "template.labels")
	if err != nil {
		return apperr.New(ErrRunRedeploy).WithCause(err)
	}
	if opName != "" {
		if err := waitOperation(ctx, accessToken, opName); err != nil {
			return apperr.New(ErrRunRedeploy).WithCause(err)
		}
	}
	return nil
}
