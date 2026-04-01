package cmd

import (
	"strings"
	"testing"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/safety"
)

func TestValidateWebhookAndSecretPatternsRejectsQuerySecret(t *testing.T) {
	t.Parallel()

	pol := safety.DefaultPolicy(safety.ProfileWebhookLowTraffic)
	env := map[string]string{
		"WEBHOOK_URL": "https://example.com/hook?secret=abc",
	}
	if err := validateWebhookAndSecretPatterns(env, pol); err == nil {
		t.Fatal("expected query-secret validation error, got nil")
	}
}

func TestValidateWebhookProtectionEnvRequiresSecretKey(t *testing.T) {
	t.Parallel()

	pol := safety.DefaultPolicy(safety.ProfileWebhookLowTraffic)
	err := validateWebhookProtectionEnv("my-webhook", map[string]string{"PORT": "8080"}, pol)
	if err == nil {
		t.Fatal("expected missing webhook secret error, got nil")
	}
	if !strings.Contains(err.Error(), "no secret env key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWebhookProtectionEnvAcceptsSensitiveKey(t *testing.T) {
	t.Parallel()

	pol := safety.DefaultPolicy(safety.ProfileWebhookLowTraffic)
	err := validateWebhookProtectionEnv("my-webhook", map[string]string{"WEBHOOK_SECRET": "x"}, pol)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCollectCloudRunGuardrailWarnings(t *testing.T) {
	t.Parallel()

	pol := safety.DefaultPolicy(safety.ProfileDefault)
	min := 1
	max := 5
	timeout := 300
	warnings := collectCloudRunGuardrailWarnings(gcprun.DeployRequest{
		MinInstances: &min,
		MaxInstances: &max,
		TimeoutSec:   &timeout,
		Memory:       "1Gi",
	}, pol)
	if len(warnings) < 3 {
		t.Fatalf("expected multiple warnings, got %d (%v)", len(warnings), warnings)
	}
}

func TestParseMemoryToMi(t *testing.T) {
	t.Parallel()

	if got, ok := parseMemoryToMi("256Mi"); !ok || got != 256 {
		t.Fatalf("expected 256Mi => 256, got %d ok=%v", got, ok)
	}
	if got, ok := parseMemoryToMi("2Gi"); !ok || got != 2048 {
		t.Fatalf("expected 2Gi => 2048, got %d ok=%v", got, ok)
	}
	if _, ok := parseMemoryToMi("abc"); ok {
		t.Fatal("expected parse failure for invalid memory")
	}
}
