package safety

import "testing"

func TestInferProfile(t *testing.T) {
	t.Parallel()

	if got := InferProfile("telegram-bot-api"); got != ProfileWebhookLowTraffic {
		t.Fatalf("expected webhook profile, got %q", got)
	}
	if got := InferProfile("my-api"); got != ProfileDefault {
		t.Fatalf("expected default profile, got %q", got)
	}
}

func TestSensitiveKeyAndQuerySecretDetection(t *testing.T) {
	t.Parallel()

	if !IsSensitiveKey("BOT_TOKEN") {
		t.Fatal("BOT_TOKEN should be sensitive")
	}
	if IsSensitiveKey("LOG_LEVEL") {
		t.Fatal("LOG_LEVEL should not be sensitive")
	}
	if !LooksLikeQuerySecret("https://example.com/hook?token=abc") {
		t.Fatal("query token should be detected as insecure")
	}
	if LooksLikeQuerySecret("https://example.com/hook?id=123") {
		t.Fatal("non-secret query should not be flagged")
	}
}

func TestRedactURLQuery(t *testing.T) {
	t.Parallel()

	got := RedactURLQuery("https://example.com/hook?token=abc&x=1")
	if got != "https://example.com/hook" {
		t.Fatalf("unexpected redacted URL: %q", got)
	}
}
