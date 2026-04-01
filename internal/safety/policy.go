package safety

import (
	"net/url"
	"regexp"
	"strings"
)

type CloudRunPolicy struct {
	MinInstances   int
	MaxInstances   int
	TimeoutSeconds int
	Memory         string
}

type WebhookPolicy struct {
	RequireAuth        bool
	AuthMode           string // "header" or "path"
	HeaderName         string
	RejectQuerySecrets bool
	IdempotencyEnabled bool
	IdempotencyTTLSec  int
	RateLimitPerMinute int
}

type BudgetPolicy struct {
	Enabled    bool
	AmountEUR  float64
	Thresholds []float64
}

type Policy struct {
	DeploymentProfile string
	CloudRun          CloudRunPolicy
	Webhook           WebhookPolicy
	Budget            BudgetPolicy
}

const (
	ProfileDefault           = "default"
	ProfileWebhookLowTraffic = "webhook-low-traffic"
)

func DefaultPolicy(profile string) Policy {
	p := strings.ToLower(strings.TrimSpace(profile))
	if p == "" {
		p = ProfileDefault
	}

	base := Policy{
		DeploymentProfile: p,
		CloudRun: CloudRunPolicy{
			MinInstances:   0,
			MaxInstances:   1,
			TimeoutSeconds: 30,
			Memory:         "256Mi",
		},
		Webhook: WebhookPolicy{
			RequireAuth:        true,
			AuthMode:           "header",
			HeaderName:         "X-Webhook-Secret",
			RejectQuerySecrets: true,
			IdempotencyEnabled: true,
			IdempotencyTTLSec:  3600,
			RateLimitPerMinute: 120,
		},
		Budget: BudgetPolicy{
			Enabled:    false,
			AmountEUR:  10,
			Thresholds: []float64{0.5, 0.9, 1.0},
		},
	}

	switch p {
	case ProfileWebhookLowTraffic:
		return base
	default:
		return base
	}
}

func InferProfile(serviceName string) string {
	s := strings.ToLower(strings.TrimSpace(serviceName))
	if strings.Contains(s, "webhook") || strings.Contains(s, "bot") || strings.Contains(s, "telegram") {
		return ProfileWebhookLowTraffic
	}
	return ProfileDefault
}

var sensitiveKeyPattern = regexp.MustCompile(`(?i)(token|secret|password|passwd|db_url|database_url|api_key|private_key|webhook)`)

func IsSensitiveKey(key string) bool {
	return sensitiveKeyPattern.MatchString(strings.TrimSpace(key))
}

func LooksLikeQuerySecret(v string) bool {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.RawQuery == "" {
		return false
	}
	for k := range u.Query() {
		key := strings.ToLower(strings.TrimSpace(k))
		if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "key") {
			return true
		}
	}
	return false
}

func RedactURLQuery(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	if u.RawQuery == "" {
		return u.String()
	}
	u.RawQuery = ""
	return u.String()
}
