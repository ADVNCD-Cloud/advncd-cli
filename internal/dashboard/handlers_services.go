package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func servicesListHandler(w http.ResponseWriter, r *http.Request) {
	render := func(vm views.ServicesListVM) {
		lctx := views.LayoutCtx{
			ProjectID:    vm.Project,
			Region:       vm.Region,
			ServiceCount: len(vm.Services),
		}
		views.Layout("Services", "services",
			[]views.Crumb{
				{Label: "Overview", Href: "/"},
				{Label: "Services", Href: ""},
			},
			lctx,
			views.ServicesList(vm),
		).
			Render(r.Context(), w)
	}

	// Load config
	cfgStore, err := config.DefaultStore()
	if err != nil {
		render(views.ServicesListVM{Error: "Failed to init config store: " + err.Error(), Now: time.Now()})
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
		render(views.ServicesListVM{
			Error: "Project not initialized. Run: advncd init",
			Now:   time.Now(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	merged, regErr := loadRegistryServiceState(cfg.ProjectID, cfg.Region)
	vm := views.ServicesListVM{
		Project: cfg.ProjectID,
		Region:  cfg.Region,
		Now:     time.Now(),
	}
	if regErr != nil {
		vm.Error = "Failed to load local registry: " + regErr.Error()
	}

	tb, err := auth.GetAccessToken(ctx)
	if err == nil {
		live, liveErr := gcprun.ListServices(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region)
		if liveErr != nil {
			if vm.Error == "" {
				vm.Error = liveErr.Error()
			}
		} else {
			merged = reconcileLiveServices(merged, cfg.ProjectID, cfg.Region, live)
		}
	} else if vm.Error == "" {
		vm.Error = "Not authenticated. Run: advncd login"
	}

	sorted := stateMapToSortedList(merged)
	rows := make([]views.ServicesRowVM, 0, len(sorted))
	for _, s := range sorted {
		escapedName := url.PathEscape(s.Name)
		logsURL, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, s.Name, "PT1H", "ALL")
		metricsURL := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, s.Name, "PT1H")
		updatedText := "—"
		if !s.UpdatedAt.IsZero() {
			updatedText = s.UpdatedAt.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, views.ServicesRowVM{
			Name:        s.Name,
			SourceType:  strings.TrimSpace(s.SourceType),
			Status:      normalizeStatus(s.Status),
			URL:         strings.TrimSpace(s.URL),
			UpdatedText: updatedText,
			DetailHref:  "/services/" + escapedName,
			LogsURL:     logsURL,
			MetricsURL:  metricsURL,
			RedeployURL: "/services/" + escapedName + "/redeploy",
			DeleteURL:   "/services/" + escapedName + "/delete",
			DisableURL:  "/services/" + escapedName + "/disable",
		})
	}

	vm.Services = rows
	queryErr := strings.TrimSpace(r.URL.Query().Get("error"))
	if queryErr != "" {
		vm.Error = queryErr
	}

	render(vm)
}
