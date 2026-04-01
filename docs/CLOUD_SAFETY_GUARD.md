# Cloud Safety & Cost Guard (Sprint v0.1)

## Objectives
- Safe-by-default Cloud Run deploys.
- Cost-controlled defaults for low-traffic services.
- Stronger webhook and secret handling with practical guardrails.

## Implemented Guardrails

### 1) Secret handling
- Deploy flows now classify sensitive env keys and sync values into Google Secret Manager.
- Cloud Run receives Secret Manager references instead of raw env values where possible.
- Sensitive key classes include token/secret/password/db_url/database_url/cron_secret patterns.

Code paths:
- `cmd/guardrails.go` (`syncSensitiveEnvToSecretManager`)
- `internal/gcpsecretmanager/secrets.go`
- `internal/gcprun/deploy.go` (`SecretEnv` / `valueSource.secretKeyRef`)

### 2) Webhook security
- Reusable middleware supports:
  - header-based secret auth
  - path-based secret auth
- Query-string secrets are explicitly rejected.
- Webhook-like deploy profiles validate protection requirements.

Code paths:
- `internal/safety/webhook.go`
- `cmd/guardrails.go`

### 3) Idempotency and retry safety
- Middleware deduplicates events via:
  - `X-Request-Id`
  - `Idempotency-Key`
  - JSON payload keys (`update_id`, `request_id`)
- Duplicate requests return success without reprocessing.

### 4) Rate limiting
- Middleware includes fixed-window per-minute rate limiting.
- Intended for low-traffic webhook services.

### 5) Cloud Run cost defaults
- Guardrail policy defaults:
  - `min-instances = 0`
  - `max-instances = 1`
  - `timeout = 30s`
  - `memory = 256Mi` (profile-dependent)
- Deploy paths apply defaults when unset and print warnings for risky overrides.

### 6) Budget bootstrap
- New CLI command:
  - `advncd budget init --amount-eur 10 --thresholds 0.5,0.9,1.0`
- Creates project budget with alert thresholds.

Code paths:
- `cmd/budget.go`
- `internal/gcpbilling/budget.go`

### 7) Kill switch
- New emergency command:
  - `advncd service disable <name>`
  - `advncd services disable <name>`
- Default mode removes unauthenticated invoker (`allUsers`).
- `--hard` deletes the service.

Code paths:
- `cmd/service.go`, `cmd/services_disable.go`
- `internal/gcprun/iam.go`

### 8) Observability and anomaly classification
- Webhook middleware emits structured request logs with:
  - `path`
  - `source_classification`
  - `status`
  - `latency_ms`
  - `request_type`
  - `event`
- Includes suspicious burst classification from repeated rate-limit hits.
- Raw secrets and query secrets are not logged.

### 9) Service traffic/cost visibility in dashboard
- Service detail page now includes **Service Safety / Traffic / Cost** section.
- Displays near-live indicators from Cloud Monitoring (best effort):
  - request volume
  - requests per minute
  - p95 latency
  - error rate
  - instance activity
- Displays cost visibility with explicit labels:
  - estimated hourly burn
  - estimated daily burn
  - today estimated cost
  - actual billed cost status (integration-aware placeholder when unavailable)
- Adds anomaly badges for:
  - request spikes vs baseline
  - elevated error rate
  - saturation risk
  - unexpected activity signals

### 10) Fast UI kill switch
- Dashboard now exposes **Disable Public** action for each service.
- This revokes unauthenticated invoker access (`allUsers`) for fast containment.

## Guardrails Config
`advncd.yaml` supports:

```yaml
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
    idempotency_ttl_seconds: 3600
    rate_limit_per_minute: 120
  budget:
    enabled: true
    amount_eur: 10
    thresholds_csv: 0.5,0.9,1.0
```

## Current Scope Limits
- Middleware is reusable but application-level route wiring is service-specific.
- Budget bootstrap currently creates budget thresholds; notification channels are not auto-provisioned in this sprint.
