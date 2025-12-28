Scenario 1 — First-time setup (0 → publish → stats)

Цель

Ты впервые запускаешь Advncd, логинишься в GCP без gcloud, деплоишь Go-приложение в Cloud Run и видишь статус/метрики в локальном Dashboard.

Шаг 1: старт локальной среды

Command
	•	advncd dev

Expected
	•	Agent и Dashboard подняты локально
	•	Dashboard открывается на http://localhost:4321

Dashboard
	•	Local: ✅ Agent up
	•	GCP: ❌ Not connected (кнопка “Copy: advncd login gcp”)

⸻

Шаг 2: логин в GCP (device flow)

Command
	•	advncd login gcp

Expected
	•	показ URL + code
	•	после подтверждения: “Logged in as you@…”

Dashboard
	•	GCP: ✅ Connected as you@…
	•	Project: ⚠️ not set

⸻

Шаг 3: выбрать проект и регион

Command
	•	advncd gcp project set my-project
	•	advncd gcp region set europe-west1

Expected
	•	“project set”
	•	“region set”

Dashboard
	•	GCP: ✅ Connected
	•	Project: ✅ my-project
	•	Region: ✅ europe-west1
	•	APIs: ⚠️ maybe missing (показывает список)

⸻

Шаг 4: publish Go app

Command
	•	advncd publish gcp apps/admin-go --service admin-go

Expected
	•	Build submitted → success
	•	Deploy → URL

Dashboard
	•	Apps → появляется admin-go:
	•	URL
	•	Ready status
	•	Last deployed timestamp
	•	Builds → запись о последнем билде + логи

⸻

Шаг 5: смотреть “статистику”

UI
	•	Apps → admin-go → вкладка “Overview”
	•	“Operational status” (ready, revisions, traffic)
	•	“Metrics” (MVP: requests; затем errors/latency)

⸻

Scenario 2 — Token revoked / expired (re-login flow)

Цель

Refresh token сломался (пользователь отозвал доступ, сменился клиент и т.д.). CLI и UI должны мягко сказать что делать.

Симптом
	•	advncd status показывает auth error

Command
	•	advncd status

Expected Output

GCP
  Auth: ❌ session expired or revoked
Fix:
  advncd login gcp
Details:
  invalid_grant

Dashboard
	•	GCP card: ⚠️ “Session revoked”
	•	CTA: “Copy: advncd login gcp”

Исправление

Command
	•	advncd login gcp

Expected
	•	снова connected

⸻

Scenario 3 — Read-only mode (смотреть Cloud Run без deploy)

Цель

Ты не хочешь деплоить, просто подключаешь аккаунт и смотришь существующий Cloud Run сервис и его метрики.

Шаг 1: login + project/region set

(как в Scenario 1)

Шаг 2: “import existing app”

Вариант A (CLI):
	•	advncd apps import admin-go --region europe-west1

Вариант B (UI):
	•	Apps → “Import service” → name + region

Expected
	•	сервис появляется в списке Apps

Dashboard
	•	показывает:
	•	Ready/URL
	•	последнюю ревизию
	•	traffic split
	•	(metrics) requests/errors/latency — если permissions позволяют

Если не хватает прав на метрики

Dashboard
	•	Metrics: ❌ Insufficient permissions
	•	Fix: попросить roles/monitoring.viewer

⸻

Scenario 4 — Missing APIs (классика первого деплоя)

(очень частый случай — стоит описать в доке)

Симптом

publish блокируется

Command
	•	advncd publish gcp apps/admin-go --service admin-go

Expected

❌ Publish blocked: required APIs not enabled
Missing:
  cloudbuild.googleapis.com
  artifactregistry.googleapis.com
Fix:
  Enable APIs in GCP Console → APIs & Services → Library

Dashboard
	•	GCP card → “APIs missing” + список

