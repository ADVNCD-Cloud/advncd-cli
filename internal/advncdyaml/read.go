package advncdyaml

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const FileName = "advncd.yaml"

func ReadFromRoot(root string) (*Config, error) {
	path := filepath.Join(strings.TrimSpace(root), FileName)
	return ReadFile(path)
}

func ReadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	var section string
	var subsection string
	var envList string

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := leadingSpaces(raw)
		if indent == 0 {
			envList = ""
			subsection = ""
			if strings.HasSuffix(trimmed, ":") {
				section = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
				continue
			}
			k, v, err := parseKV(trimmed)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			switch k {
			case "version":
				n, err := strconv.Atoi(unquote(v))
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid version", path, lineNo)
				}
				cfg.Version = n
			}
			continue
		}

		if section == "env" {
			if indent == 2 && strings.HasSuffix(trimmed, ":") {
				envList = strings.TrimSuffix(trimmed, ":")
				continue
			}
			if indent >= 4 && strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				item = unquote(item)
				if item == "" {
					continue
				}
				switch envList {
				case "required":
					cfg.Env.Required = append(cfg.Env.Required, item)
				case "optional":
					cfg.Env.Optional = append(cfg.Env.Optional, item)
				}
				continue
			}
		}

		if section == "guardrails" {
			if indent == 2 && strings.HasSuffix(trimmed, ":") {
				subsection = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
				continue
			}

			k, v, err := parseKV(trimmed)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			val := unquote(v)

			switch {
			case indent == 2:
				switch k {
				case "deployment_profile":
					cfg.Guardrails.DeploymentProfile = val
				}
			case indent >= 4:
				switch subsection {
				case "cloud_run":
					switch k {
					case "min_instances":
						n, err := strconv.Atoi(val)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.cloud_run.min_instances", path, lineNo)
						}
						cfg.Guardrails.CloudRun.MinInstances = n
					case "max_instances":
						n, err := strconv.Atoi(val)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.cloud_run.max_instances", path, lineNo)
						}
						cfg.Guardrails.CloudRun.MaxInstances = n
					case "timeout_seconds":
						n, err := strconv.Atoi(val)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.cloud_run.timeout_seconds", path, lineNo)
						}
						cfg.Guardrails.CloudRun.TimeoutSeconds = n
					case "memory":
						cfg.Guardrails.CloudRun.Memory = val
					}
				case "webhook":
					switch k {
					case "require_auth":
						b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(val)))
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.webhook.require_auth", path, lineNo)
						}
						cfg.Guardrails.Webhook.RequireAuth = b
					case "auth_mode":
						cfg.Guardrails.Webhook.AuthMode = val
					case "secret_header":
						cfg.Guardrails.Webhook.SecretHeader = val
					case "reject_query_secrets":
						b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(val)))
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.webhook.reject_query_secrets", path, lineNo)
						}
						cfg.Guardrails.Webhook.RejectQuerySecrets = b
					case "idempotency_enabled":
						b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(val)))
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.webhook.idempotency_enabled", path, lineNo)
						}
						cfg.Guardrails.Webhook.IdempotencyEnabled = b
					case "idempotency_ttl_seconds":
						n, err := strconv.Atoi(val)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.webhook.idempotency_ttl_seconds", path, lineNo)
						}
						cfg.Guardrails.Webhook.IdempotencyTTLSec = n
					case "rate_limit_per_minute":
						n, err := strconv.Atoi(val)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.webhook.rate_limit_per_minute", path, lineNo)
						}
						cfg.Guardrails.Webhook.RateLimitPerMinute = n
					}
				case "budget":
					switch k {
					case "enabled":
						b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(val)))
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.budget.enabled", path, lineNo)
						}
						cfg.Guardrails.Budget.Enabled = b
					case "amount_eur":
						f, err := strconv.ParseFloat(val, 64)
						if err != nil {
							return nil, fmt.Errorf("%s:%d: invalid guardrails.budget.amount_eur", path, lineNo)
						}
						cfg.Guardrails.Budget.AmountEUR = f
					case "thresholds_csv":
						cfg.Guardrails.Budget.ThresholdsCSV = val
					}
				}
			}
			continue
		}

		if indent < 2 {
			continue
		}
		k, v, err := parseKV(trimmed)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		val := unquote(v)

		switch section {
		case "service":
			switch k {
			case "name":
				cfg.Service.Name = val
			case "display_name":
				cfg.Service.DisplayName = val
			case "type":
				cfg.Service.Type = val
			case "port":
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid service.port", path, lineNo)
				}
				cfg.Service.Port = n
			}
		case "deploy":
			switch k {
			case "project":
				cfg.Deploy.Project = val
			case "region":
				cfg.Deploy.Region = val
			case "allow_service_rename":
				b, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(val)))
				if err != nil {
					return nil, fmt.Errorf("%s:%d: invalid deploy.allow_service_rename", path, lineNo)
				}
				cfg.Deploy.AllowServiceRename = b
			}
		case "build":
			switch k {
			case "strategy":
				cfg.Build.Strategy = val
			case "start_command":
				cfg.Build.StartCommand = val
			}
		case "runtime":
			switch k {
			case "family":
				cfg.Runtime.Family = val
			case "framework":
				cfg.Runtime.Framework = val
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func leadingSpaces(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			break
		}
		n++
	}
	return n
}

func parseKV(line string) (string, string, error) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", fmt.Errorf("invalid line (expected key: value)")
	}
	key := strings.TrimSpace(line[:i])
	val := strings.TrimSpace(line[i+1:])
	if key == "" {
		return "", "", fmt.Errorf("invalid line (empty key)")
	}
	return key, val, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
