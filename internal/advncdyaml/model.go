package advncdyaml

type Config struct {
	Version int `json:"version"`

	Service    ServiceConfig    `json:"service"`
	Deploy     DeployConfig     `json:"deploy"`
	Build      BuildConfig      `json:"build"`
	Runtime    RuntimeConfig    `json:"runtime"`
	Env        EnvConfig        `json:"env"`
	Guardrails GuardrailsConfig `json:"guardrails"`
}

type ServiceConfig struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	Port        int    `json:"port"`
}

type DeployConfig struct {
	Project            string `json:"project"`
	Region             string `json:"region"`
	AllowServiceRename bool   `json:"allow_service_rename"`
}

type BuildConfig struct {
	Strategy     string `json:"strategy"`
	StartCommand string `json:"start_command"`
}

type RuntimeConfig struct {
	Family    string `json:"family"`
	Framework string `json:"framework"`
}

type EnvConfig struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type GuardrailsConfig struct {
	DeploymentProfile string                   `json:"deployment_profile"`
	CloudRun          GuardrailsCloudRunConfig `json:"cloud_run"`
	Webhook           GuardrailsWebhookConfig  `json:"webhook"`
	Budget            GuardrailsBudgetConfig   `json:"budget"`
}

type GuardrailsCloudRunConfig struct {
	MinInstances   int    `json:"min_instances"`
	MaxInstances   int    `json:"max_instances"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Memory         string `json:"memory"`
}

type GuardrailsWebhookConfig struct {
	RequireAuth        bool   `json:"require_auth"`
	AuthMode           string `json:"auth_mode"`
	SecretHeader       string `json:"secret_header"`
	RejectQuerySecrets bool   `json:"reject_query_secrets"`
	IdempotencyEnabled bool   `json:"idempotency_enabled"`
	IdempotencyTTLSec  int    `json:"idempotency_ttl_seconds"`
	RateLimitPerMinute int    `json:"rate_limit_per_minute"`
}

type GuardrailsBudgetConfig struct {
	Enabled       bool    `json:"enabled"`
	AmountEUR     float64 `json:"amount_eur"`
	ThresholdsCSV string  `json:"thresholds_csv"`
}
