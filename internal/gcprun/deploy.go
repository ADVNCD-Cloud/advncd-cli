package gcprun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrRunGet    = apperr.E("C-RUN-001", "Failed to fetch Cloud Run service")
	ErrRunDeploy = apperr.E("C-RUN-002", "Failed to deploy Cloud Run service")
)

type DeployRequest struct {
	AccessToken string
	ProjectID   string
	Region      string
	ServiceName string
	Image       string
}

type DeployResult struct {
	URL string
}

// Cloud Run v2 service representation (minimal)
type service struct {
	Name string `json:"name,omitempty"`
	URI  string `json:"uri,omitempty"`
	Template struct {
		Containers []struct {
			Image string `json:"image"`
			Ports []struct {
				ContainerPort int `json:"containerPort,omitempty"`
			} `json:"ports,omitempty"`
		} `json:"containers"`
	} `json:"template,omitempty"`
}

type opLike struct {
	Name string `json:"name"`
	Done bool   `json:"done,omitempty"`
}

func DeployService(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	exists, current, err := getService(ctx, req)
	if err != nil {
		return nil, err
	}

	if !exists {
		opName, err := createService(ctx, req)
		if err != nil {
			return nil, err
		}
		if opName != "" {
			if err := waitOperation(ctx, req.AccessToken, opName); err != nil {
				return nil, err
			}
		}
		svc, err := fetchService(ctx, req)
		if err != nil {
			return nil, err
		}
		return &DeployResult{URL: svc.URI}, nil
	}

	// update existing
	current.Template.Containers = []struct {
		Image string `json:"image"`
		Ports []struct {
			ContainerPort int `json:"containerPort,omitempty"`
		} `json:"ports,omitempty"`
	}{
		{
			Image: req.Image,
			Ports: []struct {
				ContainerPort int `json:"containerPort,omitempty"`
			}{{ContainerPort: 8080}},
		},
	}

	opName, err := patchService(ctx, req, current)
	if err != nil {
		return nil, err
	}
	if opName != "" {
		if err := waitOperation(ctx, req.AccessToken, opName); err != nil {
			return nil, err
		}
	}
	svc, err := fetchService(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DeployResult{URL: svc.URI}, nil
}

func serviceURL(req DeployRequest) string {
	return fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/services/%s", req.ProjectID, req.Region, req.ServiceName)
}

func getService(ctx context.Context, req DeployRequest) (bool, *service, error) {
	u := serviceURL(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, nil, apperr.New(ErrRunGet).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return false, nil, apperr.New(ErrRunGet).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode == 404 {
		return false, nil, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, nil, apperr.New(ErrRunGet).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw))
	}

	var out service
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, nil, apperr.New(ErrRunGet).WithCause(err).
			WithMeta("raw_body", string(raw))
	}
	return true, &out, nil
}

func fetchService(ctx context.Context, req DeployRequest) (*service, error) {
	exists, svc, err := getService(ctx, req)
	if err != nil {
		return nil, err
	}
	if !exists || svc == nil {
		return nil, apperr.New(ErrRunGet).
			WithMeta("service", req.ServiceName).
			WithFix("Service was not found after deployment; check Cloud Run console.")
	}
	return svc, nil
}

func createService(ctx context.Context, req DeployRequest) (string, error) {
	u, _ := url.Parse(fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/services", req.ProjectID, req.Region))
	q := u.Query()
	q.Set("serviceId", req.ServiceName)
	u.RawQuery = q.Encode()

	payload := service{}
	payload.Template.Containers = []struct {
		Image string `json:"image"`
		Ports []struct {
			ContainerPort int `json:"containerPort,omitempty"`
		} `json:"ports,omitempty"`
	}{
		{
			Image: req.Image,
			Ports: []struct {
				ContainerPort int `json:"containerPort,omitempty"`
			}{{ContainerPort: 8080}},
		},
	}

	b, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return "", apperr.New(ErrRunDeploy).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return "", apperr.New(ErrRunDeploy).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", apperr.New(ErrRunDeploy).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Run API is enabled and you have permission to deploy.")
	}

	// Cloud Run v2 returns a long-running operation on create/update.
	var op opLike
	if err := json.Unmarshal(raw, &op); err == nil && op.Name != "" {
		return op.Name, nil
	}

	// If it returned service directly (rare), no op to wait.
	return "", nil
}

func patchService(ctx context.Context, req DeployRequest, current *service) (string, error) {
	u, _ := url.Parse(serviceURL(req))
	q := u.Query()
	q.Set("updateMask", "template.containers")
	u.RawQuery = q.Encode()

	b, _ := json.Marshal(current)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(b))
	if err != nil {
		return "", apperr.New(ErrRunDeploy).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return "", apperr.New(ErrRunDeploy).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", apperr.New(ErrRunDeploy).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Run API is enabled and you have permission to deploy.")
	}

	var op opLike
	if err := json.Unmarshal(raw, &op); err == nil && op.Name != "" {
		return op.Name, nil
	}
	return "", nil
}