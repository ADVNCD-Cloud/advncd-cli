package registry

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
)

func TestStore_UpsertByServiceIdentity(t *testing.T) {
	t.Parallel()

	s := &Store{Path: filepath.Join(t.TempDir(), "registry.json")}

	first := contracts.DeploymentRecord{
		SourceType:   contracts.SourceTypeCode,
		ServiceName:  "svc",
		ProjectID:    "proj",
		Region:       "europe-west3",
		URL:          "https://a",
		Status:       "ready",
		LastRevision: "rev-1",
	}
	if err := s.UpsertDeploymentByServiceIdentity(first); err != nil {
		t.Fatalf("first upsert error: %v", err)
	}

	got1, err := s.ResolveDeploymentByServiceIdentity(contracts.ServiceIdentity{
		ServiceName: "svc",
		ProjectID:   "proj",
		Region:      "europe-west3",
	})
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if got1 == nil {
		t.Fatal("expected record, got nil")
	}

	second := *got1
	second.URL = "https://b"
	second.Status = "ready"
	second.LastRevision = "rev-2"
	time.Sleep(1 * time.Millisecond)
	if err := s.UpsertDeploymentByServiceIdentity(second); err != nil {
		t.Fatalf("second upsert error: %v", err)
	}

	got2, err := s.ResolveDeploymentByServiceIdentity(contracts.ServiceIdentity{
		ServiceName: "svc",
		ProjectID:   "proj",
		Region:      "europe-west3",
	})
	if err != nil {
		t.Fatalf("resolve2 error: %v", err)
	}
	if got2 == nil {
		t.Fatal("expected record, got nil")
	}
	if got2.DeploymentID != got1.DeploymentID {
		t.Fatalf("deployment id changed: %q vs %q", got2.DeploymentID, got1.DeploymentID)
	}
	if got2.URL != "https://b" {
		t.Fatalf("url not updated: %q", got2.URL)
	}
}
