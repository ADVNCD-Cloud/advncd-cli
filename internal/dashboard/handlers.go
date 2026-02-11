package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func overviewHandler(w http.ResponseWriter, r *http.Request) {
	render := func(vm views.HomeVM) {
		views.Layout("Overview", "overview",
			[]views.Crumb{
				{Label: "Overview", Href: ""},
			},
			views.Home(vm),
		).Render(r.Context(), w)
	}

	vm := views.HomeVM{
		Now:           time.Now(),
		Email:         "-",
		TokenExpiresIn: "-",
		ProjectID:      "-",
		Region:         "-",
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
		vm.ProjectStatus = "missing"
		vm.ProjectHint = "Run: advncd init"
		render(vm)
		return
	}
	vm.ProjectStatus = "ok"
	vm.ProjectID = cfg.ProjectID
	vm.Region = cfg.Region
	vm.ConfigPath = cfgStore.Path

	// Load creds
	credStore, err := creds.DefaultStore()
	if err != nil {
		vm.Error = "Failed to init creds store: " + err.Error()
		render(vm)
		return
	}
	cr, err := credStore.Load()
	if err != nil || cr == nil || cr.AccessToken == "" {
		vm.AuthStatus = "missing"
		vm.AuthHint = "Run: advncd login"
		vm.CredsPath = credStore.Path
		render(vm)
		return
	}

	vm.AuthStatus = "ok"
	vm.Email = cr.Email
	vm.CredsPath = credStore.Path

	if !cr.Expiry.IsZero() {
		d := time.Until(cr.Expiry).Round(time.Second)
		if d < 0 {
			vm.AuthStatus = "expired"
			vm.AuthHint = "Run: advncd login"
			vm.TokenExpiresIn = "expired"
		} else {
			vm.TokenExpiresIn = d.String()
		}
	}

	// Cloud Run snapshot (top 7 services)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	svcs, err := gcprun.ListServices(ctx, cr.AccessToken, cfg.ProjectID, cfg.Region)
	if err != nil {
		// Keep auth/context visible, but show error banner
		vm.Error = err.Error()
		render(vm)
		return
	}

	vm.ServicesTotal = len(svcs)
	for _, s := range svcs {
		if s.Status == "READY" {
			vm.ServicesReady++
		} else {
			vm.ServicesIssues++
		}
	}

	limit := 7
	if len(svcs) < limit {
		limit = len(svcs)
	}

	rows := make([]views.HomeServiceRowVM, 0, limit)
	for i := 0; i < limit; i++ {
		rows = append(rows, views.HomeServiceRowVM{
			Name:   svcs[i].Name,
			Status: svcs[i].Status,
			URL:    svcs[i].URL,
		})
	}
	vm.Services = rows

	render(vm)
}