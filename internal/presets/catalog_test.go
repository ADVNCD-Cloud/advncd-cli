package presets

import "testing"

func TestListContainsExpectedPresetIDs(t *testing.T) {
	t.Parallel()

	got := List()
	if len(got) < 6 {
		t.Fatalf("expected at least 6 presets, got %d", len(got))
	}

	required := []string{
		PresetN8N,
		PresetTelegramBot,
		PresetWebhook,
		PresetStrapi,
		PresetSimpleAPI,
		PresetStaticSite,
	}

	for _, id := range required {
		if _, ok := FindByID(id); !ok {
			t.Fatalf("preset %q not found", id)
		}
	}
}

func TestFindLaunchSpecForImplementedNonN8NPresets(t *testing.T) {
	t.Parallel()

	ids := []string{
		PresetTelegramBot,
		PresetWebhook,
		PresetStrapi,
		PresetSimpleAPI,
		PresetStaticSite,
	}

	for _, id := range ids {
		spec, ok := FindLaunchSpec(id)
		if !ok {
			t.Fatalf("launch spec missing for %q", id)
		}
		if spec.Image == "" {
			t.Fatalf("launch image missing for %q", id)
		}
		if spec.ContainerPort <= 0 {
			t.Fatalf("container port missing for %q", id)
		}
	}
}

func TestStrapiLaunchSpecUsesRealImage(t *testing.T) {
	t.Parallel()

	spec, ok := FindLaunchSpec(PresetStrapi)
	if !ok {
		t.Fatalf("launch spec missing for %q", PresetStrapi)
	}
	if spec.Image == "" {
		t.Fatalf("launch image missing for %q", PresetStrapi)
	}
	if spec.Image == defaultDemoImage {
		t.Fatalf("strapi launch image must not be demo image")
	}
}
