package publishplan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteFile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "advncd.deploy.yaml")

	min := 1
	in := Plan{
		Version:              Version,
		Service:              "my-api",
		Stack:                StackNextJS,
		SourceDir:            ".",
		BuildMethod:          BuildMethodBuildpacks,
		ImageRepo:            "advncd",
		Port:                 8080,
		Memory:               "1Gi",
		MinInstances:         &min,
		AllowUnauthenticated: true,
		EnvFile:              ".env.production",
		Env: map[string]string{
			"NODE_ENV": "production",
			"LOG":      "debug",
		},
	}

	if err := WriteFile(path, in); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	out, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if out.Service != in.Service {
		t.Fatalf("service mismatch: got %q want %q", out.Service, in.Service)
	}
	if out.Stack != in.Stack {
		t.Fatalf("stack mismatch: got %q want %q", out.Stack, in.Stack)
	}
	if out.Port != in.Port {
		t.Fatalf("port mismatch: got %d want %d", out.Port, in.Port)
	}
	if out.Memory != in.Memory {
		t.Fatalf("memory mismatch: got %q want %q", out.Memory, in.Memory)
	}
	if out.MinInstances == nil || *out.MinInstances != min {
		t.Fatalf("min_instances mismatch")
	}
	if out.EnvFile != in.EnvFile {
		t.Fatalf("env_file mismatch: got %q want %q", out.EnvFile, in.EnvFile)
	}
	if out.Env["NODE_ENV"] != "production" || out.Env["LOG"] != "debug" {
		t.Fatalf("env mismatch: %+v", out.Env)
	}
}

func TestReadFile_InvalidLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "advncd.deploy.yaml")
	if err := os.WriteFile(path, []byte("service no-colon\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := ReadFile(path); err == nil {
		t.Fatal("expected error")
	}
}
