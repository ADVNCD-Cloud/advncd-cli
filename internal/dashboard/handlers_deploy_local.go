package dashboard

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/detect"
)

func deployLocalHandler(w http.ResponseWriter, r *http.Request) {
	render := func(vm views.DeployLocalVM) {
		views.Layout("Deploy Local Project", "overview",
			[]views.Crumb{
				{Label: "Overview", Href: "/"},
				{Label: "Deploy Local Project", Href: ""},
			},
			views.DeployLocal(vm),
		).Render(r.Context(), w)
	}

	vm := views.DeployLocalVM{
		Now:        time.Now(),
		Path:       ".",
		Service:    "app",
		Runtime:    "unknown",
		Build:      "manual",
		Port:       8080,
		Confidence: "low",
		Project:    "-",
		Region:     "-",
	}

	cfgStore, err := config.DefaultStore()
	if err == nil {
		cfg, loadErr := cfgStore.Load()
		if loadErr == nil && cfg != nil {
			if strings.TrimSpace(cfg.ProjectID) != "" {
				vm.Project = strings.TrimSpace(cfg.ProjectID)
			}
			if strings.TrimSpace(cfg.Region) != "" {
				vm.Region = strings.TrimSpace(cfg.Region)
			}
		}
	}

	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path = "."
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		vm.Error = err.Error()
		render(vm)
		return
	}
	vm.Path = absPath

	detected, err := detect.Project(absPath)
	if err != nil {
		vm.Error = err.Error()
		render(vm)
		return
	}

	vm.Service = strings.TrimSpace(detected.ServiceProposal)
	if vm.Service == "" {
		vm.Service = "app"
	}
	vm.Runtime = detected.Profile.Runtime
	vm.Build = detected.Profile.BuildStrategy
	vm.Port = detected.Profile.Port
	vm.Confidence = detected.Profile.Confidence
	vm.Warnings = append([]string(nil), detected.Profile.Warnings...)

	render(vm)
}
