package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/advncdyaml"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
)

func TestApplyYAMLOverrides(t *testing.T) {
	t.Parallel()

	base := contracts.DeployProfile{
		Runtime:       "unknown",
		BuildStrategy: "buildpacks",
		StartStrategy: "buildpack_default",
		Port:          8080,
	}
	cfg := &advncdyaml.Config{}
	cfg.Runtime.Family = "node"
	cfg.Runtime.Framework = "express"
	cfg.Build.Strategy = "buildpacks"
	cfg.Build.StartCommand = "npm start"
	cfg.Service.Port = 3001
	cfg.Env.Required = []string{"DB_URL"}

	got := applyYAMLOverrides(base, cfg)
	if got.Runtime != "node" {
		t.Fatalf("runtime mismatch: %q", got.Runtime)
	}
	if got.Framework != "express" {
		t.Fatalf("framework mismatch: %q", got.Framework)
	}
	if got.StartStrategy != "explicit" {
		t.Fatalf("start strategy mismatch: %q", got.StartStrategy)
	}
	if got.Port != 3001 {
		t.Fatalf("port mismatch: %d", got.Port)
	}
}

func TestApplyYAMLOverrides_IgnoresUnknownRuntimePlaceholder(t *testing.T) {
	t.Parallel()

	base := contracts.DeployProfile{
		Runtime:       "node",
		BuildStrategy: "buildpacks",
		Port:          8080,
		Confidence:    "high",
	}
	cfg := &advncdyaml.Config{}
	cfg.Runtime.Family = "unknown"

	got := applyYAMLOverrides(base, cfg)
	if got.Runtime != "node" {
		t.Fatalf("runtime should keep detected value, got %q", got.Runtime)
	}
}

func TestNormalizeProfileAfterOverrides_UnknownBecomesLowConfidence(t *testing.T) {
	t.Parallel()

	profile := contracts.DeployProfile{
		Runtime:       "unknown",
		BuildStrategy: "buildpacks",
		Port:          8080,
		Confidence:    "high",
	}

	got := normalizeProfileAfterOverrides(profile)
	if got.Confidence != "low" {
		t.Fatalf("confidence mismatch: got %q want %q", got.Confidence, "low")
	}
}

func TestWriteDetectedYAML_WritesProfileFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	profile := contracts.DeployProfile{
		Runtime:       "go",
		Framework:     "",
		BuildStrategy: "buildpacks",
		Port:          8080,
	}

	path, err := writeDetectedYAML(root, nil, profile, "svc", "my-project", "europe-west3")
	if err != nil {
		t.Fatalf("writeDetectedYAML error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	cfg, err := advncdyaml.ReadFile(filepath.Join(root, advncdyaml.FileName))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Service.Name != "svc" || cfg.Service.Port != 8080 {
		t.Fatalf("service mismatch: %#v", cfg.Service)
	}
	if cfg.Deploy.Project != "my-project" || cfg.Deploy.Region != "europe-west3" {
		t.Fatalf("deploy mismatch: %#v", cfg.Deploy)
	}
	if cfg.Build.Strategy != "buildpacks" {
		t.Fatalf("build strategy mismatch: %q", cfg.Build.Strategy)
	}
	if cfg.Runtime.Family != "go" {
		t.Fatalf("runtime mismatch: %q", cfg.Runtime.Family)
	}
}

func TestWriteDetectedYAML_PreservesExistingDeployTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	current := &advncdyaml.Config{
		Version: 1,
		Deploy: advncdyaml.DeployConfig{
			Project: "existing-project",
			Region:  "existing-region",
		},
	}
	profile := contracts.DeployProfile{
		Runtime:       "python",
		BuildStrategy: "buildpacks",
		Port:          8080,
	}

	_, err := writeDetectedYAML(root, current, profile, "svc", "", "")
	if err != nil {
		t.Fatalf("writeDetectedYAML error: %v", err)
	}

	cfg, err := advncdyaml.ReadFile(filepath.Join(root, advncdyaml.FileName))
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Deploy.Project != "existing-project" || cfg.Deploy.Region != "existing-region" {
		t.Fatalf("deploy target was not preserved: %#v", cfg.Deploy)
	}
}

func TestResolveServiceNamePrecedence(t *testing.T) {
	t.Parallel()

	cfg := &advncdyaml.Config{}
	cfg.Service.Name = "yaml-service"

	if got := resolveServiceName("flag-service", cfg, "proposal"); got != "flag-service" {
		t.Fatalf("flag should win, got %q", got)
	}
	if got := resolveServiceName("", cfg, "proposal"); got != "yaml-service" {
		t.Fatalf("yaml should win over proposal, got %q", got)
	}
	if got := resolveServiceName("", nil, "proposal"); got != "proposal" {
		t.Fatalf("proposal should be used, got %q", got)
	}
}
