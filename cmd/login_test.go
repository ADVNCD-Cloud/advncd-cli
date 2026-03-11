package cmd

import (
	"testing"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/authbroker"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

func TestResolveAuthBaseURL_Precedence(t *testing.T) {
	existing := &creds.Credentials{AuthBaseURL: "https://stored.example.com"}

	t.Setenv("ADVNCD_AUTH_BASE_URL", "https://env.example.com")

	got := resolveAuthBaseURL(existing, "https://flag.example.com")
	if got != "https://flag.example.com" {
		t.Fatalf("flag must win: got %q", got)
	}

	got = resolveAuthBaseURL(existing, "")
	if got != "https://env.example.com" {
		t.Fatalf("env must win over stored/default: got %q", got)
	}

	t.Setenv("ADVNCD_AUTH_BASE_URL", "")

	got = resolveAuthBaseURL(existing, "")
	if got != "https://stored.example.com" {
		t.Fatalf("stored must win over default: got %q", got)
	}

	got = resolveAuthBaseURL(nil, "")
	if got != authbroker.DefaultBaseURL {
		t.Fatalf("default must be used when nothing else exists: got %q", got)
	}
}
