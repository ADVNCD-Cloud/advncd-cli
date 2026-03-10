package dashboard

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func serviceDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/services/")
	path = strings.Trim(path, "/")
	name, ok := serviceNameFromPath(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	render := func(vm views.ServiceDetailVM) {
		views.Layout("Service "+name, "services",
			[]views.Crumb{
				{Label: "Overview", Href: "/"},
				{Label: "Services", Href: "/services"},
				{Label: name, Href: ""},
			},
			views.ServiceDetail(vm),
		).
			Render(r.Context(), w)
	}

	// Load config
	cfgStore, err := config.DefaultStore()
	if err != nil {
		render(views.ServiceDetailVM{Error: "Failed to init config store: " + err.Error(), Name: name, Now: time.Now()})
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
		render(views.ServiceDetailVM{
			Error: "Project not initialized. Run: advncd init",
			Name:  name,
			Now:   time.Now(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		render(views.ServiceDetailVM{
			Error:   "Not authenticated. Run: advncd login",
			Name:    name,
			Project: cfg.ProjectID,
			Region:  cfg.Region,
			Now:     time.Now(),
		})
		return
	}

	// IMPORTANT: GetService returns *gcprun.ServiceDetail in your codebase
	svc, err := gcprun.GetService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name)
	if err != nil {
		render(views.ServiceDetailVM{
			Error:   err.Error(),
			Name:    name,
			Project: cfg.ProjectID,
			Region:  cfg.Region,
			Now:     time.Now(),
		})
		return
	}

	// Observability presets
	logs1h, logsQuery := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "PT1H", "ALL")
	logs6h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "PT6H", "ALL")
	logs24h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "P1D", "ALL")
	logsErr1h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "PT1H", "ERROR")

	metrics1h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "PT1H")
	metrics6h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "PT6H")
	metrics24h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, svc.Name, "P1D")

	authStatus := "ok"
	authHint := ""
	if !tb.Expiry.IsZero() && time.Until(tb.Expiry) <= 0 {
		authStatus = "expired"
		authHint = "Run: advncd login"
	}

	render(views.ServiceDetailVM{
		Name:       svc.Name,
		Project:    cfg.ProjectID,
		Region:     cfg.Region,
		URL:        svc.URL,
		Image:      svc.Image,
		Conditions: mapConditions(svc.Conditions),

		ConsoleURL: cloudRunConsoleURL(cfg.ProjectID, cfg.Region, svc.Name),

		LogsURL1h:  logs1h,
		LogsURL6h:  logs6h,
		LogsURL24h: logs24h,
		LogsURLErr: logsErr1h,
		LogsQuery:  logsQuery,

		MetricsURL1h:  metrics1h,
		MetricsURL6h:  metrics6h,
		MetricsURL24h: metrics24h,

		Now: time.Now(),

		AuthStatus: authStatus,
		AuthHint:   authHint,
	})
}
