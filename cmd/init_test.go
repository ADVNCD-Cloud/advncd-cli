package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureAdvncdYAML_CreatesFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	written, path, err := ensureAdvncdYAML(root, "my-project", "europe-west3")
	if err != nil {
		t.Fatalf("ensureAdvncdYAML error: %v", err)
	}
	if !written {
		t.Fatal("expected file to be written")
	}
	if !strings.HasSuffix(path, "advncd.yaml") {
		t.Fatalf("unexpected path: %s", path)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated yaml: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "project: my-project") {
		t.Fatalf("generated yaml missing project: %s", s)
	}
	if !strings.Contains(s, "region: europe-west3") {
		t.Fatalf("generated yaml missing region: %s", s)
	}
}

func TestEnsureAdvncdYAML_DoesNotOverwrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "advncd.yaml")
	original := "version: 1\nservice:\n  name: keep-me\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write existing yaml: %v", err)
	}

	written, gotPath, err := ensureAdvncdYAML(root, "new-project", "us-central1")
	if err != nil {
		t.Fatalf("ensureAdvncdYAML error: %v", err)
	}
	if written {
		t.Fatal("expected no write when file already exists")
	}
	if gotPath != path {
		t.Fatalf("path mismatch: got %s want %s", gotPath, path)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	if string(b) != original {
		t.Fatalf("existing yaml was modified")
	}
}
