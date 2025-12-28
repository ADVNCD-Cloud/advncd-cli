## Командное дерево advncd (v0 → v1)

Core
	•	advncd dev
	•	стартует Agent + Dashboard
	•	печатает URL и локальный статус
	•	advncd status
	•	сводка: local (agent/dashboard), cloud (gcp auth/project/apis)

Auth / Cloud (GCP)
	•	advncd login gcp
	•	device flow (код + ссылка)
	•	сохраняет refresh token
	•	advncd logout gcp
	•	удаляет локальные креды
	•	advncd whoami
	•	локально показывает: user email, project, region

GCP config
	•	advncd gcp project list (опционально, если успеем)
	•	advncd gcp project set <PROJECT_ID>
	•	advncd gcp region set <REGION> (default можно europe-west1)
	•	advncd gcp apis check
	•	проверяет включены ли нужные APIs
	•	печатает команды/ссылки как включить

Publish (v1)
	•	advncd publish gcp <path> --service <name> [--region <r>]
	•	build → deploy → сохранить deployment
	•	advncd apps list
	•	advncd apps describe <name>
	•	advncd apps metrics <name> (позже, UI тоже)

Важный принцип UX: любая команда, если чего-то не хватает, не падает молча, а печатает “Fix with:” и готовую команду.

⸻

## Локальные файлы (чётко фиксируем)

2.1 Конфиг (не секреты)

Путь: ~/.advncd/config.json

{
  "version": 1,
  "gcp": {
    "projectId": null,
    "region": "europe-west1"
  },
  "agent": {
    "host": "127.0.0.1",
    "port": 4545
  },
  "dashboard": {
    "port": 4321
  }
}

2.2 Credentials (секреты)

Путь: ~/.advncd/credentials.json

{
  "version": 1,
  "gcp": {
    "provider": "google",
    "clientId": "…apps.googleusercontent.com",
    "scopes": ["https://www.googleapis.com/auth/cloud-platform"],
    "refreshToken": "…",
    "accessToken": "…",
    "accessTokenExpiresAt": "2025-12-28T10:00:00Z",
    "userEmail": "you@example.com",
    "userId": "1234567890"
  }
}

Правила:
	•	refreshToken — основной секрет.
	•	accessToken кэшируем, но всегда умеем обновить через refresh.
	•	v0 хранение в файле ок; v1 можно вынести в OS keychain.

2.3 Local session (для связи Dashboard ↔ Agent)

Путь: ~/.advncd/session.json

{
  "version": 1,
  "agentUrl": "http://127.0.0.1:4545",
  "agentToken": "random-128bit",
  "createdAt": "2025-12-28T10:00:00Z"
}


⸻

## Минимальные “Cloud capabilities” (какие API мы будем звать)

Это нужно, чтобы понимать, что мы проверяем в status и что нужно для publish.

Read-only (сразу полезно)
	•	Cloud Run Admin API: описать сервис, получить URL/Ready.
	•	Service Usage API: проверить, включены ли нужные APIs.
	•	Cloud Monitoring API: метрики (позже).

Для publish (v1)
	•	Cloud Build API: запуск сборки
	•	Artifact Registry API: пуш образов (через build)
	•	Cloud Run Admin API: update service to new image

⸻

4) Что покажет advncd status (пример вывода)
	•	Local
	•	Agent: ✅ up (http://127.0.0.1:4545)
	•	Dashboard: ✅ up (http://localhost:4321)
	•	GCP
	•	Auth: ✅ connected as you@example.com
	•	Project: ⚠️ not set (fix: advncd gcp project set <PROJECT_ID>)
	•	Region: europe-west1
	•	APIs:
	•	run.googleapis.com ✅
	•	cloudbuild.googleapis.com ❌ (fix: enable via Console / позже через API)
	•	artifactregistry.googleapis.com ✅

⸻

## Минимальный набор ручек агента (чтобы dashboard был тонким)

Local status / session
	•	GET /health
	•	GET /session → agentUrl + token present (без утечек)

GCP status
	•	GET /gcp/status
	•	connected? email?
	•	project set?
	•	region?
	•	apis enabled?

(позже) publish pipeline
	•	POST /gcp/publish → запускает build+deploy и стримит логи (SSE)

⸻

6) Следующие микро-задачи (с чего начать прямо сейчас)
	1.	advncd login gcp (device flow) → записывает credentials.json
	2.	advncd whoami → читает creds, обновляет access token при необходимости
	3.	advncd gcp project set → пишет config.json
	4.	advncd status → сводка local+cloud
	5.	Dashboard карточка “GCP” → читает /gcp/status от агента

