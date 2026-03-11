# Advncd CLI

Advncd CLI is a local-first developer tool for shipping workloads to Google Cloud Run without relying on `gcloud` in your daily loop.

It combines authentication, project setup, deploy, operations links, and n8n provisioning into one CLI.

## Why Advncd

- Faster cloud onboarding: log in once, keep local credentials, and stop copy-pasting setup commands.
- Opinionated deploy path: build and publish a Go service to Cloud Run in one command.
- One-command n8n hosting: deploy or redeploy n8n with Cloud Run-safe defaults.
- Better operations workflow: open logs, metrics, service details, and local dashboard instantly.
- Human-readable errors: structured messages with clear fixes.

## What You Can Do

- Authenticate via browser through the Advncd OAuth broker backend.
- Persist and auto-refresh app tokens, then mint short-lived GCP access tokens on demand.
- Pick or create a GCP project for n8n deployment.
- Deploy Go apps to Cloud Run through Cloud Build + Artifact Registry.
- Manage Cloud Run services (list/describe/open/logs/metrics).
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

3. Generate deploy plan and deploy your app (any stack supported by Buildpacks):

```bash
./advncd publish scan
./advncd publish
```

4. Check service status:

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

#### `advncd publish`
Build and deploy current app to Cloud Run using `advncd.deploy.yaml`.

Requirements:
- Project and region must be set (`advncd init`).
- If `advncd.deploy.yaml` is missing, CLI starts a setup wizard and creates it.

Flags:
- `--plan`: path to deploy plan YAML (default `advncd.deploy.yaml`).
- `--name`: Cloud Run service name override.
- `--env-file`: dotenv file with runtime variables (overrides `env_file` from plan at deploy time; also can be used with `publish scan` to set plan `env_file`).
- `--env`: extra `KEY=VALUE` values (repeatable). Overrides plan env and `--env-file`.

#### `advncd publish scan`
Scan current project and generate `advncd.deploy.yaml`.

`env_file` behavior:
- Auto-detects `env_file` from project root with priority: `.env.production` -> `.env.prod` -> `.env`.
- You can force plan value: `advncd publish scan --env-file .env.staging`.

Auto-detects stack heuristically:
- Frontend SSR: Next.js, Nuxt, SvelteKit, Remix, Astro, Angular SSR
- Frontend SPA: Vite-based apps
- Node backends: NestJS, Express, Fastify, Koa, Hono
- Python backends: FastAPI, Flask, Django
- Go backends: Gin, Echo, Fiber
- JVM backends: Spring Boot, Quarkus, Micronaut, Ktor
- Others: .NET, Rails/Sinatra, Laravel/Symfony, Rust, Elixir
- Fallbacks: node/python/java/php/ruby/unknown

Full matrix with detection markers:
- [`docs/STACK_MATRIX.md`](docs/STACK_MATRIX.md)

Examples:

```bash
./advncd publish scan
./advncd publish
./advncd publish --plan ./deploy/prod.yaml
./advncd publish scan --force --name api
./advncd publish scan --force --env-file .env.staging
./advncd publish --env-file .env.production
./advncd publish --env FOO=bar --env LOG_LEVEL=info
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
- `--memory`: memory limit (default `2Gi`).
- `--port`: container port (default `5678`).
- `--min-instances`: Cloud Run min instances (default `1`, use `-1` to keep current).
- `--no-cpu-throttling`: disable CPU throttling (default `true`).
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

### LLM

#### `advncd llm status`
Show active LLM provider/model/base URL and basic connectivity check for Ollama.

### Dashboard

#### `advncd dashboard`
Run local dashboard HTTP server and open it in browser.

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

File permissions are restricted by the CLI (`0700` directory, `0600` files where applicable).

Recommended:
- Never commit `.env` with secrets.
- Use a stable external Postgres and `N8N_ENCRYPTION_KEY` for production n8n.

## Typical Flows

Go service deploy:

```bash
./advncd login
./advncd init
./advncd publish
./advncd services
```

n8n managed deploy:

```bash
./advncd login
./advncd n8n
./advncd n8n --redeploy
```

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
