package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/advncdyaml"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpsecretmanager"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/safety"
)

func resolveGuardrailsPolicy(serviceName string, yamlCfg *advncdyaml.Config) safety.Policy {
	profile := safety.InferProfile(serviceName)
	if yamlCfg != nil && strings.TrimSpace(yamlCfg.Guardrails.DeploymentProfile) != "" {
		profile = strings.ToLower(strings.TrimSpace(yamlCfg.Guardrails.DeploymentProfile))
	}
	pol := safety.DefaultPolicy(profile)
	if yamlCfg == nil {
		return pol
	}
	if yamlCfg.Guardrails.CloudRun.MinInstances >= 0 {
		pol.CloudRun.MinInstances = yamlCfg.Guardrails.CloudRun.MinInstances
	}
	if yamlCfg.Guardrails.CloudRun.MaxInstances > 0 {
		pol.CloudRun.MaxInstances = yamlCfg.Guardrails.CloudRun.MaxInstances
	}
	if yamlCfg.Guardrails.CloudRun.TimeoutSeconds > 0 {
		pol.CloudRun.TimeoutSeconds = yamlCfg.Guardrails.CloudRun.TimeoutSeconds
	}
	if strings.TrimSpace(yamlCfg.Guardrails.CloudRun.Memory) != "" {
		pol.CloudRun.Memory = strings.TrimSpace(yamlCfg.Guardrails.CloudRun.Memory)
	}

	if yamlCfg.Guardrails.Webhook.RequireAuth {
		pol.Webhook.RequireAuth = true
	}
	if strings.TrimSpace(yamlCfg.Guardrails.Webhook.AuthMode) != "" {
		pol.Webhook.AuthMode = strings.ToLower(strings.TrimSpace(yamlCfg.Guardrails.Webhook.AuthMode))
	}
	if strings.TrimSpace(yamlCfg.Guardrails.Webhook.SecretHeader) != "" {
		pol.Webhook.HeaderName = strings.TrimSpace(yamlCfg.Guardrails.Webhook.SecretHeader)
	}
	if yamlCfg.Guardrails.Webhook.RejectQuerySecrets {
		pol.Webhook.RejectQuerySecrets = true
	}
	if yamlCfg.Guardrails.Webhook.IdempotencyEnabled {
		pol.Webhook.IdempotencyEnabled = true
	}
	if yamlCfg.Guardrails.Webhook.IdempotencyTTLSec > 0 {
		pol.Webhook.IdempotencyTTLSec = yamlCfg.Guardrails.Webhook.IdempotencyTTLSec
	}
	if yamlCfg.Guardrails.Webhook.RateLimitPerMinute > 0 {
		pol.Webhook.RateLimitPerMinute = yamlCfg.Guardrails.Webhook.RateLimitPerMinute
	}

	if yamlCfg.Guardrails.Budget.Enabled {
		pol.Budget.Enabled = true
	}
	if yamlCfg.Guardrails.Budget.AmountEUR > 0 {
		pol.Budget.AmountEUR = yamlCfg.Guardrails.Budget.AmountEUR
	}
	if strings.TrimSpace(yamlCfg.Guardrails.Budget.ThresholdsCSV) != "" {
		pol.Budget.Thresholds = parseThresholdsCSV(yamlCfg.Guardrails.Budget.ThresholdsCSV, pol.Budget.Thresholds)
	}
	return pol
}

func applyCloudRunGuardrails(req gcprun.DeployRequest, pol safety.Policy) gcprun.DeployRequest {
	min := pol.CloudRun.MinInstances
	max := pol.CloudRun.MaxInstances
	timeout := pol.CloudRun.TimeoutSeconds

	if req.MinInstances == nil {
		req.MinInstances = intPtr(min)
	}
	if req.MaxInstances == nil {
		req.MaxInstances = intPtr(max)
	}
	if req.TimeoutSec == nil {
		req.TimeoutSec = intPtr(timeout)
	}
	if strings.TrimSpace(req.Memory) == "" {
		req.Memory = strings.TrimSpace(pol.CloudRun.Memory)
	}
	if req.CPUIDle == nil {
		req.CPUIDle = boolPtr(true)
	}
	return req
}

func validateWebhookAndSecretPatterns(env map[string]string, pol safety.Policy) error {
	for k, v := range env {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		if val == "" {
			continue
		}
		if pol.Webhook.RejectQuerySecrets && safety.LooksLikeQuerySecret(val) {
			return fmt.Errorf("unsafe secret pattern: %s contains query-string secret. Move secret to header/path and Secret Manager", key)
		}
	}
	return nil
}

func syncSensitiveEnvToSecretManager(ctx context.Context, accessToken, projectID, serviceName string, env map[string]string) (map[string]string, map[string]gcprun.SecretEnvRef, error) {
	outPlain := map[string]string{}
	outSecret := map[string]gcprun.SecretEnvRef{}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := env[k]
		if strings.TrimSpace(v) == "" {
			outPlain[k] = v
			continue
		}

		if !safety.IsSensitiveKey(k) {
			outPlain[k] = v
			continue
		}

		secretID := secretIDForEnv(serviceName, k)
		if err := gcpsecretmanager.EnsureSecretVersion(ctx, accessToken, projectID, secretID, v); err != nil {
			return nil, nil, err
		}
		outSecret[k] = gcprun.SecretEnvRef{Secret: secretID, Version: "latest"}
	}
	return outPlain, outSecret, nil
}

func validateWebhookProtectionConfig(serviceName string, yamlCfg *advncdyaml.Config, pol safety.Policy) error {
	if pol.DeploymentProfile != safety.ProfileWebhookLowTraffic || !pol.Webhook.RequireAuth {
		return nil
	}
	if yamlCfg == nil {
		return fmt.Errorf("webhook safety guard: missing advncd.yaml for %s. Add guardrails.webhook + required secret env keys", serviceName)
	}
	for _, k := range yamlCfg.Env.Required {
		if safety.IsSensitiveKey(k) {
			return nil
		}
	}
	for _, k := range yamlCfg.Env.Optional {
		if safety.IsSensitiveKey(k) {
			return nil
		}
	}
	return fmt.Errorf("webhook safety guard: %s is webhook-like but no secret env key is declared in advncd.yaml env.required/env.optional", serviceName)
}

func validateWebhookProtectionEnv(serviceName string, env map[string]string, pol safety.Policy) error {
	if pol.DeploymentProfile != safety.ProfileWebhookLowTraffic || !pol.Webhook.RequireAuth {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(pol.Webhook.AuthMode))
	if mode != "header" && mode != "path" {
		return fmt.Errorf("webhook safety guard: invalid auth_mode %q (expected header or path)", pol.Webhook.AuthMode)
	}
	if mode == "header" && strings.TrimSpace(pol.Webhook.HeaderName) == "" {
		return fmt.Errorf("webhook safety guard: auth_mode=header requires guardrails.webhook.secret_header")
	}
	for k := range env {
		if safety.IsSensitiveKey(k) {
			return nil
		}
	}
	return fmt.Errorf("webhook safety guard: %s is webhook-like but no secret env key is set (expected keys like WEBHOOK_SECRET, BOT_TOKEN, CRON_SECRET, DB_URL)", serviceName)
}

func collectCloudRunGuardrailWarnings(req gcprun.DeployRequest, pol safety.Policy) []string {
	warnings := []string{}
	if req.MinInstances != nil && *req.MinInstances > pol.CloudRun.MinInstances {
		warnings = append(warnings, fmt.Sprintf("min-instances=%d increases idle cost (guardrail default=%d)", *req.MinInstances, pol.CloudRun.MinInstances))
	}
	if req.MaxInstances != nil && *req.MaxInstances > pol.CloudRun.MaxInstances {
		warnings = append(warnings, fmt.Sprintf("max-instances=%d can increase spend under load (guardrail default=%d)", *req.MaxInstances, pol.CloudRun.MaxInstances))
	}
	if req.TimeoutSec != nil && *req.TimeoutSec > 120 {
		warnings = append(warnings, fmt.Sprintf("timeout=%ds is high for low-traffic services (recommended <= 120s)", *req.TimeoutSec))
	}
	if memMi, ok := parseMemoryToMi(req.Memory); ok && memMi > 512 {
		warnings = append(warnings, fmt.Sprintf("memory=%s may be oversized for low-traffic profile (recommended <= 512Mi)", strings.TrimSpace(req.Memory)))
	}
	return warnings
}

func parseMemoryToMi(raw string) (int, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return 0, false
	}
	if strings.HasSuffix(v, "mi") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(v, "mi")))
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	}
	if strings.HasSuffix(v, "gi") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(v, "gi")))
		if err != nil || n <= 0 {
			return 0, false
		}
		return n * 1024, true
	}
	return 0, false
}

func secretIDForEnv(serviceName, key string) string {
	svc := projectslug.Slugify(serviceName)
	k := projectslug.Slugify(strings.ToLower(strings.TrimSpace(key)))
	base := strings.TrimSpace("advncd-" + svc + "-" + k)
	if len(base) > 255 {
		base = base[:255]
	}
	return strings.Trim(base, "-")
}

func intPtr(v int) *int   { return &v }
func boolPtr(v bool) *bool { return &v }

func parseThresholdsCSV(csv string, fallback []float64) []float64 {
	parts := strings.Split(csv, ",")
	out := make([]float64, 0, len(parts))
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var v float64
		_, err := fmt.Sscanf(raw, "%f", &v)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
