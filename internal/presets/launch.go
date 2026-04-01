package presets

import (
	"fmt"
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
)

type LaunchSpec struct {
	Image         string
	ContainerPort int
	Memory        string
	Env           map[string]string
}

const defaultDemoImage = "us-docker.pkg.dev/cloudrun/container/hello"
const strapiImage = "naskio/strapi:latest"

func FindLaunchSpec(presetID string) (LaunchSpec, bool) {
	switch strings.ToLower(strings.TrimSpace(presetID)) {
	case PresetTelegramBot:
		return LaunchSpec{
			Image:         defaultDemoImage,
			ContainerPort: 8080,
			Memory:        "256Mi",
			Env:           map[string]string{},
		}, true
	case PresetWebhook:
		return LaunchSpec{
			Image:         defaultDemoImage,
			ContainerPort: 8080,
			Memory:        "256Mi",
			Env:           map[string]string{},
		}, true
	case PresetStrapi:
		return LaunchSpec{
			Image:         strapiImage,
			ContainerPort: 1337,
			Memory:        "1Gi",
			Env:           map[string]string{},
		}, true
	case PresetSimpleAPI:
		return LaunchSpec{
			Image:         defaultDemoImage,
			ContainerPort: 8080,
			Memory:        "256Mi",
			Env:           map[string]string{},
		}, true
	case PresetStaticSite:
		return LaunchSpec{
			Image:         defaultDemoImage,
			ContainerPort: 8080,
			Memory:        "256Mi",
			Env:           map[string]string{},
		}, true
	default:
		return LaunchSpec{}, false
	}
}

func DefaultServiceName(presetID, projectID string) string {
	p := strings.ToLower(strings.TrimSpace(presetID))
	if p == "" {
		p = "preset"
	}
	short := shortProjectID(projectID)
	name := fmt.Sprintf("%s-%s", p, short)
	name = projectslug.Slugify(name)
	if name == "" {
		return p
	}
	return name
}

func shortProjectID(projectID string) string {
	s := projectslug.Slugify(projectID)
	if s == "" {
		return "project"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
