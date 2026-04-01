package advncdyaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile_ParsesCoreOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	content := `
version: 1
service:
  name: my-service
  port: 3000
deploy:
  project: my-project
  region: europe-west3
build:
  strategy: buildpacks
  start_command: npm start
runtime:
  family: node
  framework: express
env:
  required:
    - DB_URL
  optional:
    - LOG_LEVEL
guardrails:
  deployment_profile: webhook-low-traffic
  cloud_run:
    min_instances: 0
    max_instances: 1
    timeout_seconds: 30
    memory: 256Mi
  webhook:
    require_auth: true
    auth_mode: header
    secret_header: X-Webhook-Secret
    reject_query_secrets: true
    idempotency_enabled: true
    idempotency_ttl_seconds: 120
    rate_limit_per_minute: 60
  budget:
    enabled: true
    amount_eur: 10
    thresholds_csv: 0.5,0.9,1.0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Version != 1 {
		t.Fatalf("version mismatch: got %d", cfg.Version)
	}
	if cfg.Service.Name != "my-service" {
		t.Fatalf("service.name mismatch: %q", cfg.Service.Name)
	}
	if cfg.Service.Port != 3000 {
		t.Fatalf("service.port mismatch: %d", cfg.Service.Port)
	}
	if cfg.Deploy.Project != "my-project" || cfg.Deploy.Region != "europe-west3" {
		t.Fatalf("deploy target mismatch: %#v", cfg.Deploy)
	}
	if cfg.Build.Strategy != "buildpacks" || cfg.Build.StartCommand != "npm start" {
		t.Fatalf("build mismatch: %#v", cfg.Build)
	}
	if cfg.Runtime.Family != "node" || cfg.Runtime.Framework != "express" {
		t.Fatalf("runtime mismatch: %#v", cfg.Runtime)
	}
	if len(cfg.Env.Required) != 1 || cfg.Env.Required[0] != "DB_URL" {
		t.Fatalf("env.required mismatch: %#v", cfg.Env.Required)
	}
	if len(cfg.Env.Optional) != 1 || cfg.Env.Optional[0] != "LOG_LEVEL" {
		t.Fatalf("env.optional mismatch: %#v", cfg.Env.Optional)
	}
	if cfg.Guardrails.DeploymentProfile != "webhook-low-traffic" {
		t.Fatalf("guardrails profile mismatch: %#v", cfg.Guardrails)
	}
	if cfg.Guardrails.CloudRun.MaxInstances != 1 || cfg.Guardrails.CloudRun.Memory != "256Mi" {
		t.Fatalf("guardrails cloud run mismatch: %#v", cfg.Guardrails.CloudRun)
	}
	if !cfg.Guardrails.Webhook.RequireAuth || cfg.Guardrails.Webhook.AuthMode != "header" {
		t.Fatalf("guardrails webhook mismatch: %#v", cfg.Guardrails.Webhook)
	}
	if !cfg.Guardrails.Budget.Enabled || cfg.Guardrails.Budget.AmountEUR != 10 {
		t.Fatalf("guardrails budget mismatch: %#v", cfg.Guardrails.Budget)
	}
}

func TestReadFile_MissingFileReturnsNil(t *testing.T) {
	t.Parallel()

	cfg, err := ReadFile(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %#v", cfg)
	}
}

func TestWriteFile_ThenReadFile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	in := &Config{
		Version: 1,
		Service: ServiceConfig{
			Name: "svc",
			Port: 8080,
		},
		Deploy: DeployConfig{
			Project:            "my-project",
			Region:             "europe-west3",
			AllowServiceRename: false,
		},
		Build: BuildConfig{
			Strategy:     "buildpacks",
			StartCommand: "npm start",
		},
		Runtime: RuntimeConfig{
			Family:    "node",
			Framework: "nextjs",
		},
		Env: EnvConfig{
			Required: []string{"DB_URL"},
			Optional: []string{"LOG_LEVEL"},
		},
		Guardrails: GuardrailsConfig{
			DeploymentProfile: "webhook-low-traffic",
			CloudRun: GuardrailsCloudRunConfig{
				MinInstances:   0,
				MaxInstances:   1,
				TimeoutSeconds: 30,
				Memory:         "256Mi",
			},
			Webhook: GuardrailsWebhookConfig{
				RequireAuth:        true,
				AuthMode:           "header",
				SecretHeader:       "X-Webhook-Secret",
				RejectQuerySecrets: true,
				IdempotencyEnabled: true,
				IdempotencyTTLSec:  120,
				RateLimitPerMinute: 60,
			},
			Budget: GuardrailsBudgetConfig{
				Enabled:       true,
				AmountEUR:     10,
				ThresholdsCSV: "0.5,0.9,1.0",
			},
		},
	}

	if err := WriteFile(path, in); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	out, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if out == nil {
		t.Fatal("expected config, got nil")
	}
	if out.Service.Name != in.Service.Name || out.Service.Port != in.Service.Port {
		t.Fatalf("service mismatch: got %#v want %#v", out.Service, in.Service)
	}
	if out.Deploy.Project != in.Deploy.Project || out.Deploy.Region != in.Deploy.Region {
		t.Fatalf("deploy mismatch: got %#v want %#v", out.Deploy, in.Deploy)
	}
	if out.Build.Strategy != in.Build.Strategy || out.Build.StartCommand != in.Build.StartCommand {
		t.Fatalf("build mismatch: got %#v want %#v", out.Build, in.Build)
	}
	if out.Runtime.Family != in.Runtime.Family || out.Runtime.Framework != in.Runtime.Framework {
		t.Fatalf("runtime mismatch: got %#v want %#v", out.Runtime, in.Runtime)
	}
	if len(out.Env.Required) != 1 || out.Env.Required[0] != "DB_URL" {
		t.Fatalf("env required mismatch: %#v", out.Env.Required)
	}
	if len(out.Env.Optional) != 1 || out.Env.Optional[0] != "LOG_LEVEL" {
		t.Fatalf("env optional mismatch: %#v", out.Env.Optional)
	}
	if out.Guardrails.DeploymentProfile != in.Guardrails.DeploymentProfile {
		t.Fatalf("guardrails profile mismatch: got %#v want %#v", out.Guardrails, in.Guardrails)
	}
	if out.Guardrails.CloudRun.Memory != in.Guardrails.CloudRun.Memory {
		t.Fatalf("guardrails cloud run mismatch: got %#v want %#v", out.Guardrails.CloudRun, in.Guardrails.CloudRun)
	}
	if out.Guardrails.Webhook.AuthMode != in.Guardrails.Webhook.AuthMode {
		t.Fatalf("guardrails webhook mismatch: got %#v want %#v", out.Guardrails.Webhook, in.Guardrails.Webhook)
	}
	if out.Guardrails.Budget.ThresholdsCSV != in.Guardrails.Budget.ThresholdsCSV {
		t.Fatalf("guardrails budget mismatch: got %#v want %#v", out.Guardrails.Budget, in.Guardrails.Budget)
	}
}
