package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func overviewHandler(w http.ResponseWriter, r *http.Request) {
	render := func(vm views.HomeVM) {
		lctx := views.LayoutCtx{
			ProjectID:      vm.ProjectID,
			Region:         vm.Region,
			Email:          vm.Email,
			TokenExpiresIn: vm.TokenExpiresIn,
			AuthStatus:     vm.AuthStatus,
			ServiceCount:   vm.ServicesTotal,
		}
		views.Layout("Overview", "overview",
			[]views.Crumb{
				{Label: "Overview", Href: ""},
			},
			lctx,
			views.Home(vm),
		).Render(r.Context(), w)
	}

	vm := views.HomeVM{
		Now:            time.Now(),
		Email:          "-",
		TokenExpiresIn: "-",
		ProjectID:      "-",
		Region:         "-",
		AuthStatus:     "missing",
		ProjectStatus:  "missing",
	}

	if projects, regions, err := loadRecentContextHints(30); err == nil {
		vm.RecentProjects = projects
		vm.RecentRegions = regions
	}

	// Load config
	cfgStore, err := config.DefaultStore()
	if err != nil {
		vm.Error = "Failed to init config store: " + err.Error()
		render(vm)
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
		vm.ProjectHint = "Run: advncd init"
		render(vm)
		return
	}
	vm.ProjectID = cfg.ProjectID
	vm.Region = cfg.Region
	vm.ConfigPath = cfgStore.Path
	vm.ProjectStatus = "ok"

	merged := map[string]serviceState{}
	registryState, regErr := loadRegistryServiceState(cfg.ProjectID, cfg.Region)
	if regErr != nil {
		vm.Error = "Failed to load local registry: " + regErr.Error()
	} else {
		merged = registryState
	}

	// Load creds
	credStore, err := creds.DefaultStore()
	if err != nil {
		vm.Error = "Failed to init creds store: " + err.Error()
		render(vm)
		return
	}
	vm.CredsPath = credStore.Path

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		vm.AuthHint = "Run: advncd login"
	} else {
		vm.AuthStatus = "ok"
		vm.Email = tb.Email
		if !tb.Expiry.IsZero() {
			d := time.Until(tb.Expiry).Round(time.Second)
			vm.TokenExpiresIn = d.String()
		}

		live, liveErr := gcprun.ListServices(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region)
		if liveErr != nil {
			if vm.Error == "" {
				vm.Error = liveErr.Error()
			}
		} else {
			merged = reconcileLiveServices(merged, cfg.ProjectID, cfg.Region, live)
		}
	}

	sorted := stateMapToSortedList(merged)
	vm.ServicesTotal = len(sorted)
	for _, s := range sorted {
		if s.Status == "ready" {
			vm.ServicesReady++
		} else {
			vm.ServicesIssues++
		}
	}

	limit := 7
	if len(sorted) < limit {
		limit = len(sorted)
	}

	rows := make([]views.HomeServiceRowVM, 0, limit)
	for i := 0; i < limit; i++ {
		s := sorted[i]
		escapedName := url.PathEscape(s.Name)
		logsURL, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, s.Name, "PT1H", "ALL")
		metricsURL := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, s.Name, "PT1H")

		updatedText := "—"
		if !s.UpdatedAt.IsZero() {
			updatedText = s.UpdatedAt.Local().Format("2006-01-02 15:04")
		}

		rows = append(rows, views.HomeServiceRowVM{
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

	render(vm)
}
