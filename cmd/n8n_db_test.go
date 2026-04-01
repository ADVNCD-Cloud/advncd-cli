package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/presets"
)

func TestStrapiPostgresEnvFromURL(t *testing.T) {
	env, err := strapiPostgresEnvFromURL(
		"postgresql://alice:secret@db.example.com:6543/strapi?sslmode=require",
		"public",
		false,
	)
	if err != nil {
		t.Fatalf("strapiPostgresEnvFromURL error: %v", err)
	}

	if env["DATABASE_CLIENT"] != "postgres" {
		t.Fatalf("DATABASE_CLIENT = %q", env["DATABASE_CLIENT"])
	}
	if env["DATABASE_HOST"] != "db.example.com" {
		t.Fatalf("DATABASE_HOST = %q", env["DATABASE_HOST"])
	}
	if env["DATABASE_PORT"] != "6543" {
		t.Fatalf("DATABASE_PORT = %q", env["DATABASE_PORT"])
	}
	if env["DATABASE_NAME"] != "strapi" {
		t.Fatalf("DATABASE_NAME = %q", env["DATABASE_NAME"])
	}
	if env["DATABASE_USERNAME"] != "alice" {
		t.Fatalf("DATABASE_USERNAME = %q", env["DATABASE_USERNAME"])
	}
	if env["DATABASE_PASSWORD"] != "secret" {
		t.Fatalf("DATABASE_PASSWORD = %q", env["DATABASE_PASSWORD"])
	}
	if env["DATABASE_SSL"] != "true" {
		t.Fatalf("DATABASE_SSL = %q", env["DATABASE_SSL"])
	}
	if env["DATABASE_SSL_REJECT_UNAUTHORIZED"] != "false" {
		t.Fatalf("DATABASE_SSL_REJECT_UNAUTHORIZED = %q", env["DATABASE_SSL_REJECT_UNAUTHORIZED"])
	}
	if env["DATABASE_SCHEMA"] != "public" {
		t.Fatalf("DATABASE_SCHEMA = %q", env["DATABASE_SCHEMA"])
	}
}

func TestStrapiPostgresEnvFromURL_SSLDisable(t *testing.T) {
	env, err := strapiPostgresEnvFromURL(
		"postgres://alice:secret@db.example.com/strapi?sslmode=disable",
		"public",
		false,
	)
	if err != nil {
		t.Fatalf("strapiPostgresEnvFromURL error: %v", err)
	}
	if env["DATABASE_SSL"] != "false" {
		t.Fatalf("DATABASE_SSL = %q", env["DATABASE_SSL"])
	}
	if _, ok := env["DATABASE_SSL_REJECT_UNAUTHORIZED"]; ok {
		t.Fatalf("DATABASE_SSL_REJECT_UNAUTHORIZED should be omitted when sslmode=disable")
	}
}

func TestResolvePresetDatabaseEnv_StrapiSQLiteDefault(t *testing.T) {
	prevDBURL := n8nDBURL
	prevSchema := n8nDBSchema
	prevReject := n8nDBSSLReject
	t.Cleanup(func() {
		n8nDBURL = prevDBURL
		n8nDBSchema = prevSchema
		n8nDBSSLReject = prevReject
	})

	n8nDBURL = ""
	n8nDBSchema = "public"
	n8nDBSSLReject = false

	env, err := resolvePresetDatabaseEnv(presets.PresetStrapi)
	if err != nil {
		t.Fatalf("resolvePresetDatabaseEnv error: %v", err)
	}
	if len(env) != 0 {
		t.Fatalf("expected empty env for sqlite default, got %v", env)
	}
}

func TestResolvePresetDatabaseEnv_RejectsUnsupportedPreset(t *testing.T) {
	prevDBURL := n8nDBURL
	prevSchema := n8nDBSchema
	prevReject := n8nDBSSLReject
	t.Cleanup(func() {
		n8nDBURL = prevDBURL
		n8nDBSchema = prevSchema
		n8nDBSSLReject = prevReject
	})

	n8nDBURL = "postgres://alice:secret@db.example.com/any?sslmode=disable"
	n8nDBSchema = "public"
	n8nDBSSLReject = false

	_, err := resolvePresetDatabaseEnv(presets.PresetWebhook)
	if err == nil {
		t.Fatal("expected error for non-strapi preset when --db-url is set")
	}
	if !strings.Contains(err.Error(), "only for `advncd launch strapi`") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStrapiRuntimeEnv_Defaults(t *testing.T) {
	env := buildStrapiRuntimeEnv()
	required := []string{
		"NODE_ENV",
		"HOST",
	}
	for _, k := range required {
		if strings.TrimSpace(env[k]) == "" {
			t.Fatalf("%s is empty", k)
		}
	}
	if env["NODE_ENV"] != "development" {
		t.Fatalf("NODE_ENV = %q", env["NODE_ENV"])
	}
	if env["HOST"] != "0.0.0.0" {
		t.Fatalf("HOST = %q", env["HOST"])
	}
	if _, ok := env["PORT"]; ok {
		t.Fatalf("PORT should not be set in user env")
	}
}

func TestBuildStrapiLaunchGuidance_MissingNodeAndNpx(t *testing.T) {
	msg := buildStrapiLaunchGuidance(func(file string) (string, error) {
		return "", errors.New("not found")
	})

	if !strings.Contains(msg, "npx create-strapi-app@latest") {
		t.Fatalf("missing npx fallback command: %s", msg)
	}
	if !strings.Contains(msg, "`node` is not installed or not found in PATH") {
		t.Fatalf("missing node warning: %s", msg)
	}
	if !strings.Contains(msg, "`npx` is not installed or not found in PATH") {
		t.Fatalf("missing npx warning: %s", msg)
	}
}

func TestBuildStrapiLaunchGuidance_ToolsPresent(t *testing.T) {
	msg := buildStrapiLaunchGuidance(func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	})

	if !strings.Contains(msg, "npx create-strapi-app@latest") {
		t.Fatalf("missing npx fallback command: %s", msg)
	}
	if strings.Contains(msg, "not found in PATH") {
		t.Fatalf("unexpected missing-tool warning: %s", msg)
	}
	if !strings.Contains(msg, "advncd launch strapi --image <your-strapi-image>") {
		t.Fatalf("missing image override fallback: %s", msg)
	}
}

func TestIsStrapiProjectRoot_TrueWhenDependencyPresent(t *testing.T) {
	root := t.TempDir()
	content := `{
  "name": "test-strapi",
  "dependencies": {
    "@strapi/strapi": "^5.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if !isStrapiProjectRoot(root) {
		t.Fatal("expected true for strapi dependency")
	}
}

func TestIsStrapiProjectRoot_FalseWhenNoDependency(t *testing.T) {
	root := t.TempDir()
	content := `{
  "name": "not-strapi",
  "dependencies": {
    "express": "^4.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if isStrapiProjectRoot(root) {
		t.Fatal("expected false when strapi dependency is absent")
	}
}
