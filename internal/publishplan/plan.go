package publishplan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
)

const (
	Version         = 1
	DefaultFileName = "advncd.deploy.yaml"

	StackUnknown = "unknown"

	// Frontend / fullstack web
	StackNextJS     = "nextjs"
	StackNuxt       = "nuxt"
	StackSvelteKit  = "sveltekit"
	StackRemix      = "remix"
	StackAstro      = "astro"
	StackAngularSSR = "angular-ssr"
	StackViteSPA    = "vite-spa"

	// Node backends
	StackNode    = "node"
	StackExpress = "express"
	StackNestJS  = "nestjs"
	StackFastify = "fastify"
	StackKoa     = "koa"
	StackHono    = "hono"

	// Python backends
	StackPython  = "python"
	StackFastAPI = "fastapi"
	StackFlask   = "flask"
	StackDjango  = "django"

	// Go backends
	StackGo    = "go"
	StackGin   = "gin"
	StackEcho  = "echo"
	StackFiber = "fiber"

	// JVM backends
	StackJava       = "java"
	StackSpringBoot = "spring-boot"
	StackQuarkus    = "quarkus"
	StackMicronaut  = "micronaut"
	StackKtor       = "ktor"

	// Other backends
	StackDotNet  = "dotnet"
	StackRails   = "rails"
	StackSinatra = "sinatra"
	StackRuby    = "ruby"
	StackLaravel = "laravel"
	StackSymfony = "symfony"
	StackPHP     = "php"
	StackRust    = "rust"
	StackElixir  = "elixir"

	BuildMethodBuildpacks = "buildpacks"
)

func SupportedStacks() []string {
	return []string{
		StackNextJS,
		StackNuxt,
		StackSvelteKit,
		StackRemix,
		StackAstro,
		StackAngularSSR,
		StackViteSPA,
		StackNode,
		StackExpress,
		StackNestJS,
		StackFastify,
		StackKoa,
		StackHono,
		StackPython,
		StackFastAPI,
		StackFlask,
		StackDjango,
		StackGo,
		StackGin,
		StackEcho,
		StackFiber,
		StackJava,
		StackSpringBoot,
		StackQuarkus,
		StackMicronaut,
		StackKtor,
		StackDotNet,
		StackRails,
		StackSinatra,
		StackRuby,
		StackLaravel,
		StackSymfony,
		StackPHP,
		StackRust,
		StackElixir,
		StackUnknown,
	}
}

type Plan struct {
	Version              int
	Service              string
	Stack                string
	SourceDir            string
	BuildMethod          string
	ImageRepo            string
	Port                 int
	Memory               string
	MinInstances         *int
	AllowUnauthenticated bool
	EnvFile              string
	Env                  map[string]string
}

func NewDefaults(serviceName string) Plan {
	serviceName = projectslug.Slugify(strings.TrimSpace(serviceName))
	if serviceName == "" {
		serviceName = "app"
	}

	return Plan{
		Version:              Version,
		Service:              serviceName,
		Stack:                StackUnknown,
		SourceDir:            ".",
		BuildMethod:          BuildMethodBuildpacks,
		ImageRepo:            "advncd",
		Port:                 8080,
		AllowUnauthenticated: true,
		Env:                  map[string]string{},
	}
}

func StackDefaults(stack string) (port int, env map[string]string) {
	switch strings.ToLower(strings.TrimSpace(stack)) {
	case StackNextJS, StackNuxt, StackSvelteKit, StackRemix, StackAstro, StackAngularSSR, StackViteSPA,
		StackNode, StackExpress, StackNestJS, StackFastify, StackKoa, StackHono:
		return 8080, map[string]string{"NODE_ENV": "production"}
	case StackLaravel:
		return 8080, map[string]string{"APP_ENV": "production", "APP_DEBUG": "false"}
	case StackSymfony:
		return 8080, map[string]string{"APP_ENV": "prod"}
	case StackRails, StackSinatra:
		return 8080, map[string]string{"RAILS_ENV": "production", "RACK_ENV": "production"}
	case StackPython, StackFastAPI, StackFlask, StackDjango:
		return 8080, map[string]string{"PYTHONUNBUFFERED": "1"}
	case StackGo, StackGin, StackEcho, StackFiber:
		return 8080, map[string]string{}
	case StackJava, StackSpringBoot, StackQuarkus, StackMicronaut, StackKtor:
		return 8080, map[string]string{}
	case StackDotNet:
		return 8080, map[string]string{}
	case StackRuby:
		return 8080, map[string]string{"RACK_ENV": "production"}
	case StackPHP:
		return 8080, map[string]string{}
	case StackRust:
		return 8080, map[string]string{}
	case StackElixir:
		return 8080, map[string]string{"MIX_ENV": "prod"}
	default:
		return 8080, map[string]string{}
	}
}

func Normalize(in Plan) (Plan, error) {
	p := in
	if p.Version == 0 {
		p.Version = Version
	}
	if p.Version != Version {
		return Plan{}, fmt.Errorf("unsupported plan version %d", p.Version)
	}

	p.Service = projectslug.Slugify(strings.TrimSpace(p.Service))
	if p.Service == "" {
		return Plan{}, fmt.Errorf("service is required")
	}

	p.Stack = normalizeStack(p.Stack)

	p.SourceDir = strings.TrimSpace(p.SourceDir)
	if p.SourceDir == "" {
		p.SourceDir = "."
	}

	p.BuildMethod = strings.ToLower(strings.TrimSpace(p.BuildMethod))
	if p.BuildMethod == "" {
		p.BuildMethod = BuildMethodBuildpacks
	}
	if p.BuildMethod != BuildMethodBuildpacks {
		return Plan{}, fmt.Errorf("unsupported build_method %q", p.BuildMethod)
	}

	p.ImageRepo = projectslug.Slugify(strings.TrimSpace(p.ImageRepo))
	if p.ImageRepo == "" {
		p.ImageRepo = "advncd"
	}

	if p.Port <= 0 {
		defPort, _ := StackDefaults(p.Stack)
		p.Port = defPort
	}

	p.Memory = strings.TrimSpace(p.Memory)
	p.EnvFile = strings.TrimSpace(p.EnvFile)

	if p.Env == nil {
		p.Env = map[string]string{}
	}
	for k := range p.Env {
		if strings.TrimSpace(k) == "" {
			return Plan{}, fmt.Errorf("env contains an empty key")
		}
	}

	if p.MinInstances != nil && *p.MinInstances < 0 {
		p.MinInstances = nil
	}

	return p, nil
}

func normalizeStack(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	for _, candidate := range SupportedStacks() {
		if s == candidate {
			return s
		}
	}
	return StackUnknown
}

func MergeEnv(base, overlay map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
