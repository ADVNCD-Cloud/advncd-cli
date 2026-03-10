# Cloud Run Stack Matrix

This matrix describes stack auto-detection currently implemented by `advncd publish scan`.

| Stack | Category | Detection markers (examples) | Default env |
|---|---|---|---|
| `nextjs` | Frontend SSR | `package.json` dep: `next` | `NODE_ENV=production` |
| `nuxt` | Frontend SSR | dep: `nuxt` | `NODE_ENV=production` |
| `sveltekit` | Frontend SSR | dep: `@sveltejs/kit` | `NODE_ENV=production` |
| `remix` | Frontend SSR | dep: `@remix-run/node` or `@remix-run/react` | `NODE_ENV=production` |
| `astro` | Frontend SSR | dep: `astro` | `NODE_ENV=production` |
| `angular-ssr` | Frontend SSR | dep: `@angular/ssr` / `@nguniversal/express-engine` / script `serve:ssr` | `NODE_ENV=production` |
| `vite-spa` | Frontend SPA | dep: `vite` + one of `react/vue/svelte/preact/solid-js` | `NODE_ENV=production` |
| `nestjs` | Node backend | dep: `@nestjs/core` | `NODE_ENV=production` |
| `express` | Node backend | dep: `express` | `NODE_ENV=production` |
| `fastify` | Node backend | dep: `fastify` or prefix `@fastify/*` | `NODE_ENV=production` |
| `koa` | Node backend | dep: `koa` | `NODE_ENV=production` |
| `hono` | Node backend | dep: `hono` | `NODE_ENV=production` |
| `node` | Node backend | fallback when only `package.json` is found | `NODE_ENV=production` |
| `fastapi` | Python backend | `requirements.txt`/`pyproject.toml` contains `fastapi` | `PYTHONUNBUFFERED=1` |
| `flask` | Python backend | python deps contain `flask` | `PYTHONUNBUFFERED=1` |
| `django` | Python backend | `manage.py` or deps contain `django` | `PYTHONUNBUFFERED=1` |
| `python` | Python backend | fallback for python project files | `PYTHONUNBUFFERED=1` |
| `gin` | Go backend | `go.mod` contains `github.com/gin-gonic/gin` | none |
| `echo` | Go backend | `go.mod` contains `github.com/labstack/echo` | none |
| `fiber` | Go backend | `go.mod` contains `github.com/gofiber/fiber` | none |
| `go` | Go backend | fallback when `go.mod` exists | none |
| `spring-boot` | JVM backend | `pom.xml`/`build.gradle*` contains `spring-boot` | none |
| `quarkus` | JVM backend | build files contain `io.quarkus` | none |
| `micronaut` | JVM backend | build files contain `micronaut` | none |
| `ktor` | JVM backend | build files contain `io.ktor` / `ktor` | none |
| `java` | JVM backend | fallback for Java build files | none |
| `dotnet` | .NET backend | `*.csproj`/`*.fsproj`/`*.vbproj`/`*.sln` in root | none |
| `rails` | Ruby backend | `Gemfile` contains `rails` | `RAILS_ENV=production`, `RACK_ENV=production` |
| `sinatra` | Ruby backend | `Gemfile` contains `sinatra` | `RAILS_ENV=production`, `RACK_ENV=production` |
| `ruby` | Ruby backend | fallback when `Gemfile` exists | `RACK_ENV=production` |
| `laravel` | PHP backend | `composer.json` require `laravel/framework` | `APP_ENV=production`, `APP_DEBUG=false` |
| `symfony` | PHP backend | composer require `symfony/framework-bundle` | `APP_ENV=prod` |
| `php` | PHP backend | fallback when `composer.json` exists | none |
| `rust` | Backend | `Cargo.toml` | none |
| `elixir` | Backend | `mix.exs` | `MIX_ENV=prod` |
| `unknown` | Fallback | no known markers | none |

Notes:
- Detection is heuristic and uses project-root markers.
- Build path is currently Buildpacks-based (`build_method: buildpacks`).
- If no plan file exists, `advncd publish` starts a setup wizard and writes `advncd.deploy.yaml`.
