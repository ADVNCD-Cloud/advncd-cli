
🎯 MVP Cloud Loop — Definition of Done (DoD)

Цель MVP

Без gcloud, только через Google APIs:

Создать Go-приложение → залогиниться → опубликовать в Cloud Run → видеть статус и базовую статистику в локальном Dashboard.

⸻

✅ DoD уровня продукта (что пользователь может сделать)

1) Локальная среда
	•	advncd dev:
	•	поднимает Agent и Dashboard
	•	Dashboard доступен в браузере
	•	Agent защищён локальным токеном
	•	В Dashboard видно:
	•	Local status: Agent/Dashboard up

⸻

2) Google login (GitHub CLI-style)
	•	advncd login gcp:
	•	Device Authorization Flow
	•	логин через браузер
	•	refresh token сохранён локально
	•	advncd whoami:
	•	показывает email
	•	Dashboard:
	•	GCP card: Connected as you@…

⸻

3) GCP configuration
	•	advncd gcp project set <PROJECT_ID>
	•	advncd gcp region set <REGION>
	•	advncd status показывает:
	•	Auth ✅
	•	Project ✅
	•	Region ✅
	•	APIs: ok / missing (с Fix)

⸻

4) Publish (API-only)
	•	advncd publish gcp apps/admin-go --service admin-go
	•	Cloud Build API:
	•	сборка контейнера (Dockerfile + Kaniko)
	•	Artifact Registry:
	•	image pushed
	•	Cloud Run Admin API:
	•	service created/updated
	•	CLI:
	•	печатает URL сервиса
	•	Dashboard:
	•	Apps → появился admin-go
	•	статус Ready/Not Ready
	•	last deployed timestamp

⸻

5) App status & metrics (read-only)
	•	Dashboard → App details:
	•	Cloud Run:
	•	URL
	•	Ready condition
	•	last revision
	•	traffic split
	•	Metrics (MVP):
	•	Requests (last 5–15 min)
	•	Если нет прав:
	•	чёткая ошибка + Fix (roles/monitoring.viewer)

⸻

6) Ошибки и DX
	•	Любая ошибка:
	•	человеко-читаемое сообщение
	•	Fix with: команда или действие
	•	advncd doctor:
	•	выдаёт один список Fix по порядку
	•	Все сценарии из предыдущего сообщения:
	•	first-time
	•	revoked token
	•	missing APIs
	•	read-only mode
— обработаны и протестированы

⸻

🧱 Epic: GCP API-only Publish (MVP)

Epic goal

Полный cloud loop без gcloud, через OAuth Device Flow + Google APIs.

⸻

🧩 Tasks (в правильном порядке)

EPIC A — Auth & Identity
	1.	Device Flow login (advncd login gcp)
	2.	Refresh token storage + access token refresh
	3.	whoami (email, scopes)
	4.	Error mapping (invalid_grant → re-login)

⸻

EPIC B — GCP Config & Status
	5.	gcp project set
	6.	gcp region set
	7.	Service Usage API:
	•	check required APIs
	8.	advncd status + /gcp/status (agent)

⸻

EPIC C — Cloud Build (API)
	9.	Upload source to Cloud Build
	10.	Build with Kaniko (Dockerfile)
	11.	Parse build status/logs
	12.	Persist build result (SQLite)

⸻

EPIC D — Cloud Run Deploy
	13.	Create/update Cloud Run service
	14.	Poll until Ready
	15.	Save deployment record (service, url, region, image)

⸻

EPIC E — Metrics (Monitoring API)
	16.	Requests metric (timeSeries)
	17.	Permission error handling
	18.	UI numbers (no charts в MVP)

⸻

EPIC F — Dashboard UX
	19.	GCP status card
	20.	Apps list
	21.	App details page
	22.	Error display (title + fix + details toggle)

⸻

🚦 Что мы сознательно НЕ делаем в MVP

(это важно зафиксировать)

❌ Google OAuth в браузере Dashboard
❌ Multi-user / remote dashboard
❌ Terraform
❌ Firebase/Auth/Data
❌ Angular / Flutter DX
❌ n8n / Sonar
❌ GraphQL
❌ Advanced metrics / tracing

⸻

🧠 Итоговое позиционирование (очень сильное)

Advncd v0 — local-first developer platform
with GitHub CLI–like Google auth
and API-only Cloud Run publishing for Go apps.

Это:
	•	реально сделать одному,
	•	выглядит как продукт, а не pet-project,
	•	идеально ложится на GCP narrative,
	•	отличный фундамент для Angular / Flutter / UI kit позже.

⸻
