package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func servicesListHandler(w http.ResponseWriter, r *http.Request) {
	render := func(vm views.ServicesListVM) {
		views.Layout("Services", "services",
			[]views.Crumb{
				{Label: "Overview", Href: "/"},
				{Label: "Services", Href: ""},
			},
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
	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		render(views.ServicesListVM{
			Error:   "Not authenticated. Run: advncd login",
			Project: cfg.ProjectID,
			Region:  cfg.Region,
			Now:     time.Now(),
		})
		return
	}

	svcs, err := gcprun.ListServices(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region)
	if err != nil {
		render(views.ServicesListVM{
			Error:   err.Error(),
			Project: cfg.ProjectID,
			Region:  cfg.Region,
			Now:     time.Now(),
		})
		return
	}

	render(views.ServicesListVM{
		Project:  cfg.ProjectID,
		Region:   cfg.Region,
		Services: svcs,
		Now:      time.Now(),
	})
}
