package oauth

// DefaultClientID enables zero-config `advncd login` for released binaries.
// It can be overridden at build time via:
// go build -ldflags "-X github.com/ADVNCD-Cloud/advncd-cli/internal/oauth.DefaultClientID=..."
// IMPORTANT: this must be a Google OAuth "Desktop app" client ID.
var DefaultClientID = "868914596823-8qvi8ucv3kad7eapfo6m37moun963i49.apps.googleusercontent.com"
