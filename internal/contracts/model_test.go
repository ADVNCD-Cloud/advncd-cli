package contracts

import (
	"reflect"
	"testing"
)

func TestSourceTypeValid(t *testing.T) {
	t.Parallel()

	if !SourceTypeCode.Valid() {
		t.Fatal("code source type must be valid")
	}
	if !SourceTypePreset.Valid() {
		t.Fatal("preset source type must be valid")
	}
	if SourceType("other").Valid() {
		t.Fatal("unknown source type must be invalid")
	}
}

func TestServiceIdentityCompleteAndKey(t *testing.T) {
	t.Parallel()

	id := ServiceIdentity{
		ServiceName: "svc",
		ProjectID:   "proj",
		Region:      "europe-west3",
	}
	if !id.Complete() {
		t.Fatal("identity should be complete")
	}
	if got, want := id.Key(), "svc:proj:europe-west3"; got != want {
		t.Fatalf("key mismatch: got %q want %q", got, want)
	}
}

func TestNewRegistryDefaults(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if r.Version != RegistryVersion {
		t.Fatalf("version mismatch: got %d want %d", r.Version, RegistryVersion)
	}
	if len(r.Deployments) != 0 {
		t.Fatalf("expected empty deployments, got %d", len(r.Deployments))
	}
}

func TestStableCLICommands(t *testing.T) {
	t.Parallel()

	got := StableCLICommands()
	want := []string{"detect", "deploy", "launch", "services", "dashboard"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stable commands mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
