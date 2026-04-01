package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProject_NodeExpressHighConfidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"express":"5.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	got, err := Project(dir)
	if err != nil {
		t.Fatalf("Project error: %v", err)
	}

	if got.Profile.Runtime != "node" {
		t.Fatalf("runtime mismatch: %q", got.Profile.Runtime)
	}
	if got.Profile.BuildStrategy != "buildpacks" {
		t.Fatalf("build strategy mismatch: %q", got.Profile.BuildStrategy)
	}
	if got.Profile.Confidence != "high" {
		t.Fatalf("confidence mismatch: %q", got.Profile.Confidence)
	}
}

func TestProject_DockerfileDetection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\nEXPOSE 9090\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	got, err := Project(dir)
	if err != nil {
		t.Fatalf("Project error: %v", err)
	}

	if got.Profile.Runtime != "dockerfile" {
		t.Fatalf("runtime mismatch: %q", got.Profile.Runtime)
	}
	if got.Profile.BuildStrategy != "dockerfile" {
		t.Fatalf("build strategy mismatch: %q", got.Profile.BuildStrategy)
	}
	if got.Profile.Port != 9090 {
		t.Fatalf("port mismatch: got %d want %d", got.Profile.Port, 9090)
	}
}
