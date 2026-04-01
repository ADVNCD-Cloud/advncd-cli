# Advncd CLI

Advncd CLI is a local-first developer tool for shipping workloads to Google Cloud Run without relying on `gcloud` in your daily loop.

It combines authentication, project setup, detection, deploy, registry persistence, operations links, and n8n provisioning into one CLI.

## Why Advncd

- Faster cloud onboarding: log in once, keep local credentials, and stop copy-pasting setup commands.
- Predictable detection + deploy path: inspect runtime/build/port/confidence before shipping.
- One-command n8n hosting: deploy or redeploy n8n with Cloud Run-safe defaults.
- Better operations workflow: open logs, metrics, service details, and local dashboard instantly.
- Stable local service memory: persist deployment records in a local registry.
- Human-readable errors: structured messages with clear fixes.

## What You Can Do

- Authenticate via browser through the Advncd OAuth broker backend.
- Persist and auto-refresh app tokens, then mint short-lived GCP access tokens on demand.
- Detect deploy profile for a local project (`advncd detect`).
- Deploy app code with explicit project path + service name (`advncd deploy --path --name`).
- Launch ready-made services with `advncd launch <preset>` (real: `n8n`, `strapi`; demo presets also available).
- Read `advncd.yaml` overrides for service/deploy/build/runtime/env requirements.
- Enforce deployment guardrails: low-cost Cloud Run defaults, webhook safety checks, and query-secret rejection.
- Sync sensitive runtime env values to Google Secret Manager during deploy flows.
- Bootstrap billing budgets and alert thresholds from CLI.
- Trigger emergency service kill switch with `advncd service disable` / `advncd services disable`.
- Pick or create a GCP project for n8n deployment.
- Manage Cloud Run services (list/describe/open/logs/metrics).
- Persist deployment records to local registry (`registry.json`) using stable service identity.
- Explain Cloud Run service health with local LLM (Ollama).
- Launch a local dashboard for service overview and troubleshooting.

## Installation

### Option 1: Build from source

```bash
git clone https://github.com/ADVNCD-Cloud/advncd-cli.git
cd advncd-cli
go build -o advncd .
```

### Option 2: Use release binaries

Download the binary for your OS/arch from GitHub Releases and add it to your `PATH`.

## Quick Start

1. Authenticate:

```bash
./advncd login
```

2. Select project and region:

```bash
./advncd init
```

`init` also scaffolds `advncd.yaml` in the current directory if it does not already exist, then runs detection and writes detected runtime/build/port into that new file.

3. Detect your project profile:

```bash
./advncd detect --path .
```

4. Deploy:

```bash
./advncd deploy --path . --name my-service
```

5. Check service status:

```bash
./advncd services
```

Optional auth broker override:

```bash
export ADVNCD_AUTH_BASE_URL="https://www.andreitazetdinov.com"
./advncd login
```

## n8n Quick Start on Cloud Run

Interactive deploy:

```bash
./advncd n8n --set-default
```

Redeploy to the saved project/region:

```bash
./advncd n8n --redeploy
```

Deploy with persistent Postgres (for example Supabase):

```bash
./advncd n8n \
  --db-url "postgresql://USER:PASSWORD@HOST:5432/postgres?sslmode=require" \
  --db-schema public \
  --db-ssl-reject-unauthorized=false \
  --encryption-key "YOUR_STABLE_N8N_ENCRYPTION_KEY"
```

Default n8n image is pinned to:

```text
n8nio/n8n:1.86.0
```

You can override it any time:

```bash
./advncd n8n --image n8nio/n8n:latest
```

## Next.js Deploy Example

From the repo root of your Next.js app:

```bash
# 1) Check detection result first
./advncd detect --path .

# 2) Deploy to Cloud Run
./advncd deploy --path . --name my-nextjs-app
```

From outside the app directory:

```bash
./advncd detect --path /absolute/path/to/nextjs-app
./advncd deploy --path /absolute/path/to/nextjs-app --name my-nextjs-app
```

If you add `advncd.yaml` in the app root, `deploy.project`, `deploy.region`, `service.name`, `service.port`, `build.strategy`, and runtime hints are applied as overrides.

## Cloud Safety & Cost Guard

Sprint goal: safe-by-default and cost-controlled-by-default deployments.

Current v0.1 behavior:
- Deploy flows apply Cloud Run defaults when unset: `min-instances=0`, `max-instances=1`, `timeout=30s`, memory profile defaults (`256Mi` for lightweight presets and safety profile).
- Sensitive env keys are automatically moved to Secret Manager references during deploy (`TOKEN`, `SECRET`, `PASSWORD`, `DB_URL`, `DATABASE_URL`, `CRON_SECRET`, etc.).
- Query-string secret patterns are blocked in env values (for example `...?token=...` / `...?secret=...`).
- Webhook-low-traffic profile requires explicit webhook protection inputs (header/path auth + secret env key), with reusable middleware in `internal/safety/webhook.go`.
- Emergency kill switch command is available: `advncd service disable <name>` (or `advncd services disable <name>`).
- Budget bootstrap command is available: `advncd budget init --amount-eur 10 --thresholds 0.5,0.9,1.0`.

Detailed notes:
- [`docs/CLOUD_SAFETY_GUARD.md`](docs/CLOUD_SAFETY_GUARD.md)

Example `advncd.yaml` guardrails block:

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

## Command Reference

### Core

#### `advncd login`
Authenticate with Google Cloud via browser-based auth broker flow.

Flags:
- `--auth-base-url`: auth broker base URL override (optional).

Example:

```bash
./advncd login --auth-base-url "https://www.andreitazetdinov.com"
```

#### `advncd status`
Show auth status (app token + broker probe), config state, and required API readiness.

#### `advncd apis enable [api ...]`
Enable required APIs in the current project. If no APIs are passed, enables default required set (`run`, `cloudbuild`, `artifactregistry`, `monitoring`).

Flags:
- `--project`: override configured project id.

#### `advncd logout`
Revoke app session in auth broker and delete local credentials.

#### `advncd init`
Set default project and region (interactive if omitted).
If `advncd.yaml` is missing in the current directory, it creates it and applies detected runtime/build/port values automatically.

Flags:
- `--project`: set project ID directly.
- `--region`: set region directly.

Example:

```bash
./advncd init --project my-project --region europe-west1
```

### Project Management

#### `advncd projects list`
List ACTIVE GCP projects available to the authenticated account.

#### `advncd projects delete <project_id>`
Request project deletion (moves project to deletion lifecycle).

Flags:
- `--yes`: skip confirmation prompt.

Example:

```bash
./advncd projects delete my-old-project --yes
```

### Deploy Services (Any Buildpacks Stack)

#### `advncd detect`
Detect deploy profile for a project and print:
- runtime
- build strategy
- port
- confidence
- warnings
- service name proposal

Flags:
- `--path`: project path (default `.`)
- `--name`: service name override for proposal output
- `--write-yaml`: write detected runtime/build/port values into `advncd.yaml`

Examples:

```bash
./advncd detect --path .
./advncd detect --path ./apps/web --name web-api
./advncd detect --path . --write-yaml
```

#### `advncd deploy`
Primary deploy flow for local app code.

Requirements:
- Project and region must be set (`advncd init`).
- If confidence is not high, CLI asks for confirmation before executing.
- Writes/updates a deployment record in local registry after successful deploy.

Flags:
- `--path`: project path to deploy (default `.`)
- `--name`: Cloud Run service name override

Full matrix with detection markers:
- [`docs/STACK_MATRIX.md`](docs/STACK_MATRIX.md)

Examples:

```bash
./advncd deploy --path .
./advncd deploy --path ./apps/api --name api-prod
```

#### `advncd publish` (legacy compatibility path)
Legacy plan-based deploy flow using `advncd.deploy.yaml` remains available for existing users.

#### `advncd launch <preset>`
Launch ready-made services through the primary launch verb.

Current preset support:
- `n8n`
- `strapi`
- `telegram-bot` (demo preset)
- `webhook` (demo preset)
- `simple-api` (demo preset)
- `static-site` (demo preset)

Examples:

```bash
./advncd launch n8n --set-default
./advncd launch n8n --redeploy
./advncd launch strapi
./advncd launch strapi --db-url "postgresql://USER:PASSWORD@HOST:5432/DBNAME?sslmode=require"
./advncd launch strapi --image naskio/strapi:latest
```

### n8n on Cloud Run

#### `advncd n8n`
Deploy or redeploy n8n with Cloud Run defaults optimized for editor connectivity.

Flags:
- `--project`: GCP project ID to use.
- `--project-name`: display name for new project.
- `--create-project`: create project from `--project` if needed.
- `--region`: Cloud Run region.
- `--service`: Cloud Run service name (default `n8n`).
- `--image`: container image (default `n8nio/n8n:1.86.0`).
- `--memory`: memory limit (default `512Mi`).
- `--port`: container port (default `5678`).
- `--min-instances`: Cloud Run min instances (default `0`, use `-1` to keep current).
- `--no-cpu-throttling`: disable CPU throttling (default `false`).
- `--redeploy`: use saved project/region and redeploy existing service.
- `--set-default`: save selected project/region as default config.
- `--public-url`: public base URL for n8n.
- `--db-url`: Postgres URL for persistent state.
- `--db-schema`: Postgres schema (default `public`).
- `--db-ssl-reject-unauthorized`: whether to validate DB SSL cert.
- `--encryption-key`: stable `N8N_ENCRYPTION_KEY`.

Examples:

```bash
./advncd n8n --project my-n8n-project --create-project --region europe-west3 --set-default
./advncd n8n --redeploy
./advncd n8n --image n8nio/n8n:latest --redeploy
```

### Cloud Run Service Operations

#### `advncd services`
List services in the configured project and region.

#### `advncd services describe <service>`
Show Cloud Run URL, image, and condition states.

#### `advncd services open <service>`
Open both public URL and Cloud Console page in browser.

#### `advncd services logs <service>`
Open Cloud Logging query for the service.

#### `advncd services metrics <service>`
Open Cloud Monitoring metrics view for the service.

#### `advncd services explain <service>`
Use LLM provider to explain service conditions and likely causes.

#### `advncd services delete <service>`
Delete a Cloud Run service.

Flags:
- `--yes`: skip confirmation prompt.

#### `advncd services disable <service>`
Emergency kill switch to stop public traffic quickly.

Behavior:
- default mode: removes `allUsers` invoker binding (service remains deployed but public access is cut off).
- `--hard`: requests full Cloud Run service deletion.

Flags:
- `--yes`: skip confirmation prompt.
- `--hard`: hard kill switch (delete service).

Alias:
- `advncd service disable <service>`

### Budget Guardrails

#### `advncd budget init`
Create a billing budget for the active project.

Flags:
- `--amount-eur`: budget amount in EUR (default `10.0`).
- `--thresholds`: csv thresholds (default `0.5,0.9,1.0`).
- `--billing-account`: explicit billing account id/name.
- `--display-name`: optional custom budget display name.

Example:

```bash
./advncd budget init --amount-eur 10 --thresholds 0.5,0.9,1.0
```

### LLM

#### `advncd llm status`
Show active LLM provider/model/base URL and basic connectivity check for Ollama.

### Dashboard

#### `advncd dashboard`
Run local dashboard HTTP server and open it in browser.

Service detail now includes:
- Service Safety / Traffic / Cost panel
- Near-live traffic indicators (request volume, RPM, latency, error rate, instance activity)
- Estimated live cost and explicit actual-billed-cost status
- Anomaly badges and quick `Disable Public` incident action

### Auth Utilities

#### `advncd auth print-access-token`
Print a valid access token for debugging/integration.

## Environment Variables

Auth:
- `ADVNCD_AUTH_BASE_URL` (default: `https://www.andreitazetdinov.com`)

LLM:
- `ADVNCD_LLM_PROVIDER` (default: `ollama`)
- `ADVNCD_LLM_MODEL` (default: `llama3.2`)
- `ADVNCD_LLM_BASE_URL` (default: `http://localhost:11434`)
- `ADVNCD_LLM_TIMEOUT` (Go duration format, example: `30s`)

The binary imports `github.com/joho/godotenv/autoload`, so if a `.env` file exists in the working directory, variables are loaded at runtime. They are not baked into the compiled binary.

## Local Data and Security

Advncd stores data under `os.UserConfigDir()/advncd`:

- `credentials.json`: auth broker app tokens + metadata.
- `config.json`: selected project/region defaults.
- `registry.json`: deployment records (local service memory for dashboard/operations).

File permissions are restricted by the CLI (`0700` directory, `0600` files where applicable).

Recommended:
- Never commit `.env` with secrets.
- Use a stable external Postgres and `N8N_ENCRYPTION_KEY` for production n8n.

## Service Identity in One Project

Services are differentiated by stable identity:

```text
service_name + project_id + region
```

What this means in practice:
- Same project/region, different `service_name` -> separate Cloud Run services.
- Same `service_name` + same project/region -> treated as the same service (redeploy/update path).
- Different region (even same service name) -> different service identity.

Use `--name` on deploy (or `service.name` in `advncd.yaml`) to keep each app separated clearly:

```bash
./advncd deploy --path ./apps/web --name web-frontend
./advncd deploy --path ./apps/api --name backend-api
```

## Typical Flows

Go service deploy:

```bash
./advncd login
./advncd init
./advncd detect --path .
./advncd deploy --path . --name my-service
./advncd services
```

n8n managed deploy:

```bash
./advncd login
./advncd n8n
./advncd n8n --redeploy
```

## License

This project is licensed under the PolyForm Noncommercial License 1.0.0.
Commercial use is not permitted. See [LICENSE](./LICENSE).
