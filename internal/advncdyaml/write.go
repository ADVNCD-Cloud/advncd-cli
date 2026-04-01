package advncdyaml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WriteToRoot(root string, cfg *Config) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("root is required")
	}
	path := filepath.Join(root, FileName)
	return path, WriteFile(path, cfg)
}

func WriteFile(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}

	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Env.Required == nil {
		cfg.Env.Required = []string{}
	}
	if cfg.Env.Optional == nil {
		cfg.Env.Optional = []string{}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("version: %d\n\n", cfg.Version))
	b.WriteString("service:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n", quoteMaybe(cfg.Service.Name)))
	b.WriteString(fmt.Sprintf("  port: %d\n\n", cfg.Service.Port))

	b.WriteString("deploy:\n")
	b.WriteString(fmt.Sprintf("  project: %s\n", quoteMaybe(cfg.Deploy.Project)))
	b.WriteString(fmt.Sprintf("  region: %s\n", quoteMaybe(cfg.Deploy.Region)))
	b.WriteString(fmt.Sprintf("  allow_service_rename: %t\n\n", cfg.Deploy.AllowServiceRename))

	b.WriteString("build:\n")
	b.WriteString(fmt.Sprintf("  strategy: %s\n", quoteMaybe(cfg.Build.Strategy)))
	if strings.TrimSpace(cfg.Build.StartCommand) != "" {
		b.WriteString(fmt.Sprintf("  start_command: %s\n", quoteMaybe(cfg.Build.StartCommand)))
	}
	b.WriteString("\n")

	b.WriteString("runtime:\n")
	b.WriteString(fmt.Sprintf("  family: %s\n", quoteMaybe(cfg.Runtime.Family)))
	b.WriteString(fmt.Sprintf("  framework: %s\n\n", quoteMaybe(cfg.Runtime.Framework)))

	b.WriteString("env:\n")
	b.WriteString("  required:\n")
	for _, v := range cfg.Env.Required {
		b.WriteString(fmt.Sprintf("    - %s\n", quoteMaybe(v)))
	}
	b.WriteString("  optional:\n")
	for _, v := range cfg.Env.Optional {
		b.WriteString(fmt.Sprintf("    - %s\n", quoteMaybe(v)))
	}
	b.WriteString("\n")

	b.WriteString("guardrails:\n")
	b.WriteString(fmt.Sprintf("  deployment_profile: %s\n", quoteMaybe(cfg.Guardrails.DeploymentProfile)))
	b.WriteString("  cloud_run:\n")
	b.WriteString(fmt.Sprintf("    min_instances: %d\n", cfg.Guardrails.CloudRun.MinInstances))
	b.WriteString(fmt.Sprintf("    max_instances: %d\n", cfg.Guardrails.CloudRun.MaxInstances))
	b.WriteString(fmt.Sprintf("    timeout_seconds: %d\n", cfg.Guardrails.CloudRun.TimeoutSeconds))
	b.WriteString(fmt.Sprintf("    memory: %s\n", quoteMaybe(cfg.Guardrails.CloudRun.Memory)))
	b.WriteString("  webhook:\n")
	b.WriteString(fmt.Sprintf("    require_auth: %t\n", cfg.Guardrails.Webhook.RequireAuth))
	b.WriteString(fmt.Sprintf("    auth_mode: %s\n", quoteMaybe(cfg.Guardrails.Webhook.AuthMode)))
	b.WriteString(fmt.Sprintf("    secret_header: %s\n", quoteMaybe(cfg.Guardrails.Webhook.SecretHeader)))
	b.WriteString(fmt.Sprintf("    reject_query_secrets: %t\n", cfg.Guardrails.Webhook.RejectQuerySecrets))
	b.WriteString(fmt.Sprintf("    idempotency_enabled: %t\n", cfg.Guardrails.Webhook.IdempotencyEnabled))
	b.WriteString(fmt.Sprintf("    idempotency_ttl_seconds: %d\n", cfg.Guardrails.Webhook.IdempotencyTTLSec))
	b.WriteString(fmt.Sprintf("    rate_limit_per_minute: %d\n", cfg.Guardrails.Webhook.RateLimitPerMinute))
	b.WriteString("  budget:\n")
	b.WriteString(fmt.Sprintf("    enabled: %t\n", cfg.Guardrails.Budget.Enabled))
	b.WriteString(fmt.Sprintf("    amount_eur: %v\n", cfg.Guardrails.Budget.AmountEUR))
	b.WriteString(fmt.Sprintf("    thresholds_csv: %s\n", quoteMaybe(cfg.Guardrails.Budget.ThresholdsCSV)))

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return nil
}

func quoteMaybe(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, " :#") {
		return fmt.Sprintf("%q", v)
	}
	return v
}
