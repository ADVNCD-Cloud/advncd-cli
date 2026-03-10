package gcprun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrRunGet    = apperr.E("C-RUN-001", "Failed to fetch Cloud Run service")
	ErrRunDeploy = apperr.E("C-RUN-002", "Failed to deploy Cloud Run service")
)

type DeployRequest struct {
	AccessToken   string
	ProjectID     string
	Region        string
	ServiceName   string
	Image         string
	Env           map[string]string
	ContainerPort int
	Memory        string
	MinInstances  *int
	CPUIDle       *bool
}

type DeployResult struct {
	URL string
}

type envVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type container struct {
	Image string `json:"image"`
	Ports []struct {
		ContainerPort int `json:"containerPort,omitempty"`
	} `json:"ports,omitempty"`
	Env       []envVar   `json:"env,omitempty"`
	Resources *resources `json:"resources,omitempty"`
}

type resources struct {
	Limits  map[string]string `json:"limits,omitempty"`
	CPUIDle *bool             `json:"cpuIdle,omitempty"`
}

type serviceScaling struct {
	MinInstanceCount int `json:"minInstanceCount,omitempty"`
}

// Cloud Run v2 service representation (minimal)
type service struct {
	Name     string          `json:"name,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Scaling  *serviceScaling `json:"scaling,omitempty"`
	Template struct {
		Containers []container `json:"containers"`
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
	current.Template.Containers = []container{
		buildContainer(req),
	}
	patchMask := "template.containers"
	if req.MinInstances != nil {
		current.Scaling = &serviceScaling{MinInstanceCount: *req.MinInstances}
		patchMask += ",scaling"
	}

	opName, err := patchService(ctx, req, current, patchMask)
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
	payload.Template.Containers = []container{
		buildContainer(req),
	}
	if req.MinInstances != nil {
		payload.Scaling = &serviceScaling{MinInstanceCount: *req.MinInstances}
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

func patchService(ctx context.Context, req DeployRequest, current *service, updateMask string) (string, error) {
	u, _ := url.Parse(serviceURL(req))
	q := u.Query()
	q.Set("updateMask", updateMask)
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

func buildContainerEnv(envMap map[string]string) []envVar {
	if len(envMap) == 0 {
		return nil
	}

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]envVar, 0, len(keys))
	for _, k := range keys {
		out = append(out, envVar{Name: k, Value: envMap[k]})
	}
	return out
}

func buildContainer(req DeployRequest) container {
	c := container{
		Image: req.Image,
		Ports: []struct {
			ContainerPort int `json:"containerPort,omitempty"`
		}{{ContainerPort: containerPortOrDefault(req.ContainerPort)}},
		Env: buildContainerEnv(req.Env),
	}

	memory := strings.TrimSpace(req.Memory)
	if memory != "" || req.CPUIDle != nil {
		r := &resources{}
		if memory != "" {
			r.Limits = map[string]string{"memory": memory}
		}
		if req.CPUIDle != nil {
			r.CPUIDle = req.CPUIDle
		}
		c.Resources = r
	}

	return c
}

func containerPortOrDefault(v int) int {
	if v > 0 {
		return v
	}
	return 8080
}
