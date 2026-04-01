package presets

import (
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
)

const (
	PresetN8N         = "n8n"
	PresetTelegramBot = "telegram-bot"
	PresetWebhook     = "webhook"
	PresetStrapi      = "strapi"
	PresetSimpleAPI   = "simple-api"
	PresetStaticSite  = "static-site"
)

var catalog = []contracts.PresetDefinition{
	{
		PresetID:           PresetN8N,
		DisplayName:        "n8n",
		Category:           "automation",
		Summary:            "Workflow automation service",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "n8n-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
	{
		PresetID:           PresetTelegramBot,
		DisplayName:        "Telegram Bot",
		Category:           "messaging",
		Summary:            "Quick bot/webhook backend",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "telegram-bot-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
	{
		PresetID:           PresetWebhook,
		DisplayName:        "Webhook Receiver",
		Category:           "integration",
		Summary:            "Simple HTTP receiver for tests and integrations",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "webhook-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
	{
		PresetID:           PresetStrapi,
		DisplayName:        "Strapi",
		Category:           "cms",
		Summary:            "Ready-made content backend",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "strapi-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
	{
		PresetID:           PresetSimpleAPI,
		DisplayName:        "Simple API",
		Category:           "backend",
		Summary:            "Minimal deployable API service",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "simple-api-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
	{
		PresetID:           PresetStaticSite,
		DisplayName:        "Static Site",
		Category:           "frontend",
		Summary:            "Deploy static content quickly",
		LaunchMode:         "prebuilt_image",
		DefaultNamePattern: "static-site-{project}",
		RequiredInputs:     []string{},
		OptionalInputs:     []string{"service_name", "region"},
	},
}

func List() []contracts.PresetDefinition {
	out := make([]contracts.PresetDefinition, len(catalog))
	copy(out, catalog)
	return out
}

func FindByID(presetID string) (contracts.PresetDefinition, bool) {
	id := strings.ToLower(strings.TrimSpace(presetID))
	for _, p := range catalog {
		if p.PresetID == id {
			return p, true
		}
	}
	return contracts.PresetDefinition{}, false
}
