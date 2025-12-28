Advncd GCP Error Catalog v0 (spec)

Формат сообщения (CLI/UI)

Title: коротко что случилось
Summary: 1 строка “что не так”
FixWith: список команд/действий (copy-paste)
Details: технические поля (code, reason, permission, endpoint, traceId)

Структура (как объект)
	•	code (стабильный код ошибки)
	•	severity (info|warn|error)
	•	title
	•	summary
	•	fixWith[] (строки)
	•	docsHint (опционально)
	•	details (объект)

⸻

A. Auth / Identity

GCP_AUTH_NOT_CONNECTED
	•	severity: error
	•	title: GCP not connected
	•	summary: No credentials found.
	•	fixWith:
	•	advncd login gcp
	•	details: { path: "~/.advncd/credentials.json" }

GCP_AUTH_REFRESH_FAILED
	•	severity: error
	•	title: GCP session expired or revoked
	•	summary: Could not refresh access token (invalid_grant).
	•	fixWith:
	•	advncd login gcp
	•	details: { oauthError: "invalid_grant" }

GCP_AUTH_SCOPE_INSUFFICIENT
	•	severity: error
	•	title: Insufficient OAuth scopes
	•	summary: Token missing required scope cloud-platform.
	•	fixWith:
	•	advncd logout gcp
	•	advncd login gcp
	•	details: { required: ["cloud-platform"], got: [...] }

⸻

B. Project / Region config

GCP_PROJECT_NOT_SET
	•	severity: warn
	•	title: GCP project not set
	•	summary: Set a default project to deploy and query resources.
	•	fixWith:
	•	advncd gcp project set <PROJECT_ID>

GCP_PROJECT_NOT_FOUND
	•	severity: error
	•	title: GCP project not found
	•	summary: Project ‘’ does not exist or is typed incorrectly.
	•	fixWith:
	•	advncd gcp project set <PROJECT_ID>
	•	(optional) advncd gcp project list

GCP_PROJECT_NO_ACCESS
	•	severity: error
	•	title: No access to project
	•	summary: Your account cannot access project ‘’.
	•	fixWith:
	•	Ask a project owner to grant access
	•	details: { projectId, status: 403 }

GCP_REGION_NOT_SET
	•	severity: warn
	•	title: GCP region not set
	•	summary: Set a default region (Cloud Run, Artifact Registry).
	•	fixWith:
	•	advncd gcp region set europe-west1

GCP_REGION_INVALID
	•	severity: error
	•	title: Invalid region
	•	summary: Region ‘’ is not recognized.
	•	fixWith:
	•	advncd gcp region set europe-west1

⸻

C. APIs enabled

GCP_APIS_MISSING
	•	severity: warn
	•	title: Required Google APIs are not enabled
	•	summary: Some services are disabled in this project.
	•	fixWith:
	•	Enable APIs in GCP Console → APIs & Services → Library
	•	details: { missing: ["run.googleapis.com", ...] }

GCP_APIS_CHECK_FAILED
	•	severity: warn
	•	title: Could not verify enabled APIs
	•	summary: Service Usage API request failed.
	•	fixWith:
	•	Try again later or check permissions: roles/serviceusage.serviceUsageViewer
	•	details: { status, reason }

⸻

D. IAM permissions

GCP_PERMISSION_DENIED
	•	severity: error
	•	title: Insufficient permissions
	•	summary: Missing permission ‘’ for operation ‘’.
	•	fixWith:
	•	Ask a project owner to grant required roles (see details)
	•	details: { permission, operation, suggestedRoles[] }

GCP_QUOTA_OR_BILLING
	•	severity: error
	•	title: Quota or billing issue
	•	summary: Operation blocked due to billing/quota.
	•	fixWith:
	•	Check Billing is enabled for project
	•	Check quotas in GCP Console
	•	details: { reason }

⸻

E. Cloud Run

RUN_SERVICE_NOT_FOUND
	•	severity: error
	•	title: Cloud Run service not found
	•	summary: Service ‘’ not found in region ‘’.
	•	fixWith:
	•	Check service name/region
	•	Deploy it first: advncd publish gcp <path> --service <name>

RUN_DEPLOY_FAILED
	•	severity: error
	•	title: Cloud Run deploy failed
	•	summary: Service update rejected.
	•	fixWith:
	•	Check image exists and permissions (run.admin, artifactregistry.reader)
	•	details: { status, errorMessage }

⸻

F. Cloud Build / Artifact Registry

BUILD_FAILED
	•	severity: error
	•	title: Build failed
	•	summary: Cloud Build returned a failure status.
	•	fixWith:
	•	Open build logs in Dashboard → Builds
	•	details: { buildId, status, logsUrl? }

AR_REPO_NOT_FOUND
	•	severity: error
	•	title: Artifact Registry repository not found
	•	summary: Repository ‘’ missing in region ‘’.
	•	fixWith:
	•	Create Artifact Registry repo in GCP Console (or use another repo)
	•	details: { repo, region }

IMAGE_PUSH_DENIED
	•	severity: error
	•	title: Cannot push image
	•	summary: Artifact Registry denied write access.
	•	fixWith:
	•	Grant roles/artifactregistry.writer
	•	details: { status: 403 }

⸻

2) Матрица операций → permissions/roles → Fix

2.1 Операции статуса (read-only)

Operation: gcp.whoami
	•	API: OAuth tokeninfo/userinfo
	•	Needs: валидный access token
	•	On fail: GCP_AUTH_NOT_CONNECTED, GCP_AUTH_REFRESH_FAILED

Operation: gcp.checkApis
	•	API: Service Usage
	•	Permission: serviceusage.services.list
	•	Role: roles/serviceusage.serviceUsageViewer
	•	Fix: “Grant Service Usage Viewer” / “Enable APIs in console”

Operation: run.describeService
	•	API: Cloud Run Admin
	•	Permission: run.services.get
	•	Role: roles/run.viewer (или run.admin)
	•	Fix: grant run.viewer

Operation: monitoring.queryMetrics
	•	API: Cloud Monitoring
	•	Permission: monitoring.timeSeries.list
	•	Role: roles/monitoring.viewer

⸻

2.2 Операции publish (build + deploy)

Operation: build.submit
	•	API: Cloud Build
	•	Permission: cloudbuild.builds.create
	•	Role: roles/cloudbuild.builds.editor

Operation: artifact.push
	•	API: Artifact Registry
	•	Permission: write to repo
	•	Role: roles/artifactregistry.writer

Operation: run.deployService
	•	API: Cloud Run Admin
	•	Permissions: run.services.create / run.services.update
	•	Role: roles/run.admin

⸻

3) “Suggested roles” (MVP набор)

Для “Go app → Cloud Run → metrics в Dashboard”:
	•	roles/run.admin
	•	roles/cloudbuild.builds.editor
	•	roles/artifactregistry.writer
	•	roles/monitoring.viewer
	•	roles/serviceusage.serviceUsageViewer

Если хочешь только смотреть (без publish):
	•	roles/run.viewer
	•	roles/monitoring.viewer
	•	roles/serviceusage.serviceUsageViewer

⸻

4) Как использовать этот каталог в коде (чтобы было механически)

Любой вызов API оборачиваем в mapper:
	1.	если 401/invalid_grant → GCP_AUTH_REFRESH_FAILED
	2.	если 403 и есть permission в сообщении → GCP_PERMISSION_DENIED + suggestedRoles по операции
	3.	если Service Usage показывает disabled → GCP_APIS_MISSING
	4.	если 404 → *_NOT_FOUND

И везде печатаем FixWith.

⸻

5) Мини-список “FixWith” команд (шаблоны)
	•	advncd login gcp
	•	advncd logout gcp && advncd login gcp
	•	advncd gcp project set <PROJECT_ID>
	•	advncd gcp region set europe-west1
	•	“Enable APIs in Console → APIs & Services → Library”
	•	“Grant roles: run.admin, cloudbuild.builds.editor, artifactregistry.writer, monitoring.viewer, serviceusage.serviceUsageViewer”

⸻
