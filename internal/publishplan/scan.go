package publishplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
)

type packageManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

type composerManifest struct {
	Require map[string]string `json:"require"`
}

func Scan(root string) (Plan, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Plan{}, fmt.Errorf("scan root is empty")
	}

	stack := detectStack(root)
	service := projectslug.FromPathBase(root)
	plan := NewDefaults(service)
	plan.Stack = stack

	_, defaultEnv := StackDefaults(stack)
	plan.Env = MergeEnv(plan.Env, defaultEnv)

	return Normalize(plan)
}

func detectStack(root string) string {
	pkg, hasPackage := readPackageManifest(root)
	composer, hasComposer := readComposerManifest(root)

	if stack := detectRubyStack(root); stack != "" {
		return stack
	}
	if stack := detectPHPStack(composer, hasComposer); stack != "" {
		return stack
	}
	if stack := detectFrontendStack(pkg, hasPackage); stack != "" {
		return stack
	}
	if stack := detectNodeBackendStack(pkg, hasPackage); stack != "" {
		return stack
	}
	if stack := detectPythonStack(root); stack != "" {
		return stack
	}
	if stack := detectGoStack(root); stack != "" {
		return stack
	}
	if stack := detectJavaStack(root); stack != "" {
		return stack
	}
	if looksLikeDotNet(root) {
		return StackDotNet
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return StackRust
	}
	if fileExists(filepath.Join(root, "mix.exs")) {
		return StackElixir
	}
	if hasPackage {
		return StackNode
	}
	if looksLikePythonProject(root) {
		return StackPython
	}
	if hasComposer {
		return StackPHP
	}
	if fileExists(filepath.Join(root, "Gemfile")) {
		return StackRuby
	}
	return StackUnknown
}

func detectFrontendStack(pkg packageManifest, hasPackage bool) string {
	if !hasPackage {
		return ""
	}

	if depExists(pkg, "next") {
		return StackNextJS
	}
	if depExists(pkg, "nuxt") {
		return StackNuxt
	}
	if depExists(pkg, "@sveltejs/kit") {
		return StackSvelteKit
	}
	if depExists(pkg, "@remix-run/node", "@remix-run/react") {
		return StackRemix
	}
	if depExists(pkg, "astro") {
		return StackAstro
	}
	if depExists(pkg, "@angular/ssr", "@nguniversal/express-engine") || scriptContains(pkg, "serve:ssr") {
		return StackAngularSSR
	}
	if depExists(pkg, "vite") && depExists(pkg, "react", "vue", "svelte", "preact", "solid-js") {
		return StackViteSPA
	}

	return ""
}

func detectNodeBackendStack(pkg packageManifest, hasPackage bool) string {
	if !hasPackage {
		return ""
	}

	if depExists(pkg, "@nestjs/core") {
		return StackNestJS
	}
	if depExists(pkg, "express") {
		return StackExpress
	}
	if depExists(pkg, "fastify") || depPrefixExists(pkg, "@fastify/") {
		return StackFastify
	}
	if depExists(pkg, "koa") {
		return StackKoa
	}
	if depExists(pkg, "hono") {
		return StackHono
	}

	return ""
}

func detectPythonStack(root string) string {
	if !looksLikePythonProject(root) {
		return ""
	}

	if fileExists(filepath.Join(root, "manage.py")) {
		return StackDjango
	}

	depsText := strings.ToLower(readFirstExistingFile(root,
		"requirements.txt",
		"pyproject.toml",
		"Pipfile",
	))

	if strings.Contains(depsText, "fastapi") {
		return StackFastAPI
	}
	if strings.Contains(depsText, "flask") {
		return StackFlask
	}
	if strings.Contains(depsText, "django") {
		return StackDjango
	}

	return StackPython
}

func detectGoStack(root string) string {
	goModPath := filepath.Join(root, "go.mod")
	if !fileExists(goModPath) {
		return ""
	}
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return StackGo
	}
	s := strings.ToLower(string(b))

	if strings.Contains(s, "github.com/gin-gonic/gin") {
		return StackGin
	}
	if strings.Contains(s, "github.com/labstack/echo") {
		return StackEcho
	}
	if strings.Contains(s, "github.com/gofiber/fiber") {
		return StackFiber
	}
	return StackGo
}

func detectJavaStack(root string) string {
	if !looksLikeJavaProject(root) {
		return ""
	}

	s := strings.ToLower(readFirstExistingFile(root, "pom.xml", "build.gradle", "build.gradle.kts"))
	if strings.Contains(s, "spring-boot") {
		return StackSpringBoot
	}
	if strings.Contains(s, "io.quarkus") {
		return StackQuarkus
	}
	if strings.Contains(s, "micronaut") {
		return StackMicronaut
	}
	if strings.Contains(s, "io.ktor") || strings.Contains(s, "ktor") {
		return StackKtor
	}
	return StackJava
}

func detectPHPStack(composer composerManifest, hasComposer bool) string {
	if !hasComposer {
		return ""
	}
	if _, ok := composer.Require["laravel/framework"]; ok {
		return StackLaravel
	}
	if _, ok := composer.Require["symfony/framework-bundle"]; ok {
		return StackSymfony
	}
	return StackPHP
}

func detectRubyStack(root string) string {
	if !fileExists(filepath.Join(root, "Gemfile")) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(root, "Gemfile"))
	if err != nil {
		return StackRuby
	}
	s := strings.ToLower(string(b))
	if strings.Contains(s, "gem 'rails'") || strings.Contains(s, "gem \"rails\"") {
		return StackRails
	}
	if strings.Contains(s, "gem 'sinatra'") || strings.Contains(s, "gem \"sinatra\"") {
		return StackSinatra
	}
	return StackRuby
}

func readPackageManifest(root string) (packageManifest, bool) {
	var pkg packageManifest
	b, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return pkg, false
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return pkg, false
	}
	return pkg, true
}

func readComposerManifest(root string) (composerManifest, bool) {
	var c composerManifest
	b, err := os.ReadFile(filepath.Join(root, "composer.json"))
	if err != nil {
		return c, false
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, false
	}
	return c, true
}

func depExists(pkg packageManifest, names ...string) bool {
	for _, n := range names {
		if _, ok := pkg.Dependencies[n]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[n]; ok {
			return true
		}
	}
	return false
}

func depPrefixExists(pkg packageManifest, prefix string) bool {
	for k := range pkg.Dependencies {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range pkg.DevDependencies {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func scriptContains(pkg packageManifest, key string) bool {
	if pkg.Scripts == nil {
		return false
	}
	_, ok := pkg.Scripts[key]
	return ok
}

func looksLikePythonProject(root string) bool {
	return fileExists(filepath.Join(root, "requirements.txt")) ||
		fileExists(filepath.Join(root, "pyproject.toml")) ||
		fileExists(filepath.Join(root, "Pipfile")) ||
		fileExists(filepath.Join(root, "manage.py"))
}

func looksLikeJavaProject(root string) bool {
	return fileExists(filepath.Join(root, "pom.xml")) ||
		fileExists(filepath.Join(root, "build.gradle")) ||
		fileExists(filepath.Join(root, "build.gradle.kts"))
}

func looksLikeDotNet(root string) bool {
	matches, _ := filepath.Glob(filepath.Join(root, "*.csproj"))
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob(filepath.Join(root, "*.fsproj"))
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob(filepath.Join(root, "*.vbproj"))
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob(filepath.Join(root, "*.sln"))
	return len(matches) > 0
}

func readFirstExistingFile(root string, names ...string) string {
	for _, name := range names {
		path := filepath.Join(root, name)
		b, err := os.ReadFile(path)
		if err == nil {
			return string(b)
		}
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !st.IsDir()
}
