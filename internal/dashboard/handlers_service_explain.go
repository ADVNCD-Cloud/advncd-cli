package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/llm"
)

func serviceExplainHandler(w http.ResponseWriter, r *http.Request) {
	name, ok := serviceNameFromPath(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	isHTMX := r.Header.Get("HX-Request") == "true"
	clear := r.URL.Query().Get("clear") == "1"

	// helper: render either full page or only explain card
	render := func(vm views.ServiceDetailVM) {
		if isHTMX {
			views.ExplainCard(vm).Render(r.Context(), w)
			return
		}

		views.Layout("Service "+name, "services",
			[]views.Crumb{
				{Label: "Overview", Href: "/"},
				{Label: "Services", Href: "/services"},
				{Label: name, Href: ""},
			},
			views.LayoutCtx{ServiceCount: -1},
			views.ServiceDetail(vm),
		).Render(r.Context(), w)
	}

	// If user clicked Clear: return card without calling LLM
	if clear {
		render(views.ServiceDetailVM{
			Name: name,
			Now:  time.Now(),
			// ExplainText empty => shows hint text
		})
		return
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

	ctxAuth, cancelAuth := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancelAuth()
	tb, err := auth.GetAccessToken(ctxAuth)
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

	// Fetch service detail (fast)
	ctxRun, cancelRun := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancelRun()

	svc, err := gcprun.GetService(ctxRun, tb.AccessToken, cfg.ProjectID, cfg.Region, name)
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

	// Observability presets (same as detail)
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

	// LLM explain (slower)
	client, llmCfg, err := llm.NewFromEnv()
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

	llmTimeout := llmCfg.Timeout
	if llmTimeout < 90*time.Second {
		llmTimeout = 90 * time.Second
	}
	ctxLLM, cancelLLM := context.WithTimeout(r.Context(), llmTimeout)
	defer cancelLLM()

	conds := make([]llm.ServiceCondition, 0, len(svc.Conditions))
	for _, c := range svc.Conditions {
		conds = append(conds, llm.ServiceCondition{Type: c.Type, State: c.State})
	}

	status := gcprun.DeriveStatus(svc.Conditions)

	explainText, err := llm.ExplainService(ctxLLM, client, llmCfg.Model, llm.ExplainServiceInput{
		Project:    cfg.ProjectID,
		Region:     cfg.Region,
		Service:    svc.Name,
		Status:     status,
		Image:      svc.Image,
		Conditions: conds,
	})
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

	// Render same page, but with ExplainText filled
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

		ExplainText: explainText,

		Now: time.Now(),

		AuthStatus: authStatus,
		AuthHint:   authHint,
	})
}
