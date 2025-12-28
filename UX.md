advncd login gcp --help

Purpose: Connect Advncd to Google Cloud using browser-based device login.

Usage
	•	advncd login gcp [--scope cloud-platform] [--no-open]

Options
	•	--scope <scope>: OAuth scope (default: cloud-platform)
	•	--no-open: don’t open browser automatically

Examples
	•	advncd login gcp
	•	advncd login gcp --no-open

Output (success)

To authenticate with Google Cloud, visit:
  https://www.google.com/device

Enter code:
  ABCD-EFGH

Waiting for authorization... done

✅ Logged in as you@example.com
Next:
  advncd gcp project set <PROJECT_ID>

Output (fail: revoked/denied)

❌ Login failed: authorization denied
Fix:
  advncd login gcp
Details:
  access_denied


⸻

advncd logout gcp --help

Usage
	•	advncd logout gcp

Behavior
	•	Removes local GCP credentials from ~/.advncd/credentials.json.

Output

✅ Logged out of Google Cloud


⸻

advncd whoami --help

Purpose: Show current identity and selected GCP project/region.

Usage
	•	advncd whoami

Output (connected)

GCP identity
  User:   you@example.com
  Project: my-project
  Region:  europe-west1
  Scopes:  cloud-platform

Output (not connected)

GCP identity
  Auth: ❌ not connected
Fix:
  advncd login gcp


⸻

advncd status --help

Purpose: Show a complete health snapshot (local + cloud).

Usage
	•	advncd status [--json] [--verbose]

Options
	•	--json: machine-readable output
	•	--verbose: include details and raw error reasons

Output (ideal)

Local
  Agent:     ✅ up (http://127.0.0.1:4545)
  Dashboard: ✅ up (http://localhost:4321)

GCP
  Auth:    ✅ connected as you@example.com
  Project: ✅ my-project
  Region:  ✅ europe-west1
  APIs:    ✅ ok

Output (project not set)

GCP
  Auth:    ✅ connected as you@example.com
  Project: ⚠️ not set
  Region:  ✅ europe-west1
Fix:
  advncd gcp project set <PROJECT_ID>

Output (APIs missing)

GCP
  Auth:    ✅ connected as you@example.com
  Project: ✅ my-project
  APIs:    ⚠️ missing: cloudbuild.googleapis.com, artifactregistry.googleapis.com
Fix:
  Enable APIs in GCP Console → APIs & Services → Library

Output (insufficient permissions)

GCP
  Auth:    ✅ connected as you@example.com
  Project: ✅ my-project
  Cloud Run: ❌ insufficient permissions
Fix:
  Ask a project owner to grant roles:
    roles/run.admin
    roles/monitoring.viewer
Details:
  missing permission: run.services.get


⸻

advncd gcp project set --help

Purpose: Set default GCP project for deploy and monitoring.

Usage
	•	advncd gcp project set <PROJECT_ID>

Examples
	•	advncd gcp project set my-project

Output (success)

✅ GCP project set to: my-project
Next:
  advncd gcp region set europe-west1

Output (no access)

❌ Cannot access project: my-project
Fix:
  Ask a project owner to grant you access to the project
Details:
  403 PERMISSION_DENIED


⸻

advncd gcp region set --help

Usage
	•	advncd gcp region set <REGION>

Output

✅ GCP region set to: europe-west1


⸻

advncd publish gcp --help

Purpose: Build and deploy an app to Cloud Run using Google APIs (no gcloud).

Usage
	•	advncd publish gcp <path> --service <name> [--region <region>] [--tag <tag>] [--dry-run]

Options
	•	--service <name>: Cloud Run service name
	•	--region <region>: override default region
	•	--tag <tag>: image tag (default: git sha or timestamp)
	•	--dry-run: validate auth/project/APIs and show plan without deploying

Examples
	•	advncd publish gcp apps/admin-go --service admin-go
	•	advncd publish gcp apps/admin-go --service admin-go --region europe-west1
	•	advncd publish gcp apps/admin-go --service admin-go --dry-run

Output (success)

Publishing to Google Cloud (Cloud Run)

Auth:    ✅ you@example.com
Project: ✅ my-project
Region:  ✅ europe-west1

Build
  ✅ submitted: build-1234
  ✅ success

Deploy
  ✅ service updated: admin-go
  ✅ url: https://admin-go-xxxxx-ew.a.run.app

Next:
  advncd apps describe admin-go

Output (fail: not connected)

❌ Publish blocked: GCP not connected
Fix:
  advncd login gcp

Output (fail: APIs missing)

❌ Publish blocked: required APIs not enabled
Missing:
  cloudbuild.googleapis.com
  artifactregistry.googleapis.com
Fix:
  Enable APIs in GCP Console → APIs & Services → Library

Output (fail: permission denied)

❌ Build failed: insufficient permissions
Fix:
  Ask a project owner to grant roles:
    roles/cloudbuild.builds.editor
    roles/artifactregistry.writer
Details:
  missing permission: cloudbuild.builds.create


⸻

advncd apps describe --help (минимум для “статистики”)

Usage
	•	advncd apps describe <service> [--region <region>]

Output (пример)

Cloud Run service
  Name:    admin-go
  Region:  europe-west1
  URL:     https://admin-go-xxxxx-ew.a.run.app
  Ready:   ✅ True
  Traffic: 100% → latest

Revision
  Last deployed: 2025-12-28T12:01:10Z
  Image:  europe-west1-docker.pkg.dev/my-project/advncd/admin-go:tag


⸻

Небольшая “рамка” для дальнейшего DX
	•	Все команды поддерживают --json (позже) для интеграций и тестов.
	•	Для ошибок используем Error Catalog коды (GCP_AUTH_NOT_CONNECTED и т.д.).
	•	В UI: показываем title/summary/fixWith, а details прячем в “Show details”.

