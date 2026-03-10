package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildPublishEnv_MergeFileAndArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "A=1\nB=2\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	got, err := buildPublishEnv(envPath, []string{"B=override", "C=3"})
	if err != nil {
		t.Fatalf("buildPublishEnv returned error: %v", err)
	}

	if got["A"] != "1" {
		t.Fatalf("A mismatch: got %q want %q", got["A"], "1")
	}
	if got["B"] != "override" {
		t.Fatalf("B mismatch: got %q want %q", got["B"], "override")
	}
	if got["C"] != "3" {
		t.Fatalf("C mismatch: got %q want %q", got["C"], "3")
	}
}

func TestBuildPublishEnv_InvalidEnvArg(t *testing.T) {
	t.Parallel()

	_, err := buildPublishEnv("", []string{"BROKEN"})
	if err == nil {
		t.Fatal("expected error for invalid env arg")
	}
}

func TestBuildPublishImage_UsesUniqueTimestampTag(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 19, 8, 30, 45, 0, time.UTC)
	got := buildPublishImage("europe-west3", "proj", "advncd", "example", now)
	want := "europe-west3-docker.pkg.dev/proj/advncd/example:20260219-083045"

	if got != want {
		t.Fatalf("image mismatch:\n got: %s\nwant: %s", got, want)
	}
}
