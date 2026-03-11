# Release Notes

## 0.2.2 - 2026-03-11

### Changed

- Replaced local Google OAuth code flow in CLI with auth broker flow.
- `advncd login` now uses broker `start + browser + poll` flow and stores app tokens locally.
- `GetAccessToken()` now refreshes app token via broker and fetches short-lived GCP access token via broker.
- Added `ADVNCD_AUTH_BASE_URL` (default: `https://www.andreitazetdinov.com`).
- `advncd logout` now calls broker logout before deleting local credentials.
- `advncd n8n` no longer overwrites default project/region unless `--set-default` is provided.

## 0.2.1 - 2026-03-11

### Changed

- `advncd login` now supports zero-config auth with built-in OAuth client defaults.
- `ADVNCD_GCP_CLIENT_ID` is now an optional override instead of required setup.

## 0.2.0 - 2026-03-10

### Added

- Two-step publish workflow:
  - `advncd publish scan` scans project stack and generates `advncd.deploy.yaml`
  - `advncd publish` / `advncd publish deploy` deploy using that YAML
- Deploy wizard fallback when `advncd.deploy.yaml` is missing.
- Expanded stack detection heuristics for modern frontend/backend ecosystems:
  - Frontend SSR: Next.js, Nuxt, SvelteKit, Remix, Astro, Angular SSR
  - Frontend SPA: Vite-based apps
  - Backend Node: NestJS, Express, Fastify, Koa, Hono
  - Backend Python: FastAPI, Flask, Django
  - Backend Go: Gin, Echo, Fiber
  - Backend JVM: Spring Boot, Quarkus, Micronaut, Ktor
  - Others: .NET, Rails/Sinatra, Laravel/Symfony, Rust, Elixir
- Deploy plan model with configurable service/image repo/port/memory/min-instances/public access/env/env-file.
- Stack support matrix document: `docs/STACK_MATRIX.md`.
- `publish scan` now auto-detects `env_file` from project root (`.env.production` -> `.env.prod` -> `.env`) and supports explicit override via `--env-file`.

### Changed

- `advncd publish` is no longer Go-only; deploy path now works from YAML and Buildpacks for any detected stack.
- Cloud Build builder image switched from deprecated `gcr.io/buildpacks/builder:v1` to `gcr.io/buildpacks/builder:google-22`.

## 0.1.0 - 2026-03-10

Initial public release of Advncd CLI focused on practical Google Cloud Run workflows and one-command n8n hosting.

### Highlights

- Local-first Google Cloud authentication with browser OAuth and token refresh.
- Opinionated Go-to-Cloud-Run deploy flow with Cloud Build + Artifact Registry.
- n8n deployment and redeploy command with Cloud Run-safe defaults.
- Cloud Run operational commands for inspect/open/logs/metrics.
- Project management helpers (list/delete) and interactive selection.
- Local dashboard and LLM-assisted service diagnostics.

### Added

- `advncd login`, `advncd logout`, `advncd status`, `advncd init`
- `advncd publish` with `--env-file` and repeatable `--env`
- `advncd services` with subcommands:
  - `describe`
  - `open`
  - `logs`
  - `metrics`
  - `explain`
- `advncd projects list` and `advncd projects delete`
- `advncd n8n` with:
  - interactive project selection
  - optional project creation
  - `--redeploy` mode
  - external Postgres support (`--db-url`, schema, SSL options)
  - stable encryption key support (`--encryption-key`)
  - overridable image (`--image`)
- `advncd dashboard`
- `advncd auth print-access-token`
- LLM config and connectivity check (`advncd llm status`)

### Changed

- n8n default image is pinned to `n8nio/n8n:1.86.0` for stable behavior.
- n8n deploy path sets Cloud Run environment defaults needed for editor/webhook reliability (including push backend/proxy hops and health endpoint).

### Fixed

- Automatic retry with hash-suffixed project IDs when requested project ID already exists globally.
- Better handling of n8n redeploys by preserving existing runtime env where possible.

### Known Notes

- Creating new projects can fail if account project quota is exhausted.
- Enabling `run.googleapis.com`, `artifactregistry.googleapis.com`, and `containerregistry.googleapis.com` requires billing enabled on the selected project.
- n8n without external Postgres will lose state across stateless/runtime resets; production setup should use managed Postgres and a stable `N8N_ENCRYPTION_KEY`.
