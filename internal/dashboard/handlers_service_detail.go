package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard/views"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/registry"
)

func serviceDetailHandler(w http.ResponseWriter, r *http.Request) {
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
	state := serviceState{
		Name:      name,
		ProjectID: cfg.ProjectID,
		Region:    cfg.Region,
		Status:    "unknown",
	}

	store, regErr := registry.DefaultStore()
	if regErr == nil {
		rec, resolveErr := store.ResolveDeploymentByServiceIdentity(contracts.ServiceIdentity{
			ServiceName: name,
			ProjectID:   cfg.ProjectID,
			Region:      cfg.Region,
		})
		if resolveErr == nil && rec != nil {
			state.SourceType = string(rec.SourceType)
			state.URL = strings.TrimSpace(rec.URL)
			state.Status = normalizeStatus(rec.Status)
			state.UpdatedAt = rec.UpdatedAt
			state.LastRevision = strings.TrimSpace(rec.LastRevision)
			state.DeploymentID = strings.TrimSpace(rec.DeploymentID)
			state.SourcePathOptional = strings.TrimSpace(rec.SourcePathOptional)
			state.PresetIDOptional = strings.TrimSpace(rec.PresetIDOptional)
		}
	}

	authStatus := "missing"
	authHint := "Run: advncd login"
	image := ""
	insights := serviceInsights{}
	var conditions []views.ConditionVM
	var handlerErr string

	tb, err := auth.GetAccessToken(ctx)
	if err == nil {
		authStatus = "ok"
		authHint = ""
		if !tb.Expiry.IsZero() && time.Until(tb.Expiry) <= 0 {
			authStatus = "expired"
			authHint = "Run: advncd login"
		}

		svc, svcErr := gcprun.GetService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name)
		if svcErr != nil {
			handlerErr = svcErr.Error()
		} else if svc != nil {
			state.Name = svc.Name
			if strings.TrimSpace(svc.URL) != "" {
				state.URL = strings.TrimSpace(svc.URL)
			}
			state.Status = normalizeStatus(svc.Status)
			state.LastRevision = strings.TrimSpace(svc.LatestRevision)
			image = svc.Image
			conditions = mapConditions(svc.Conditions)
			insights = buildServiceInsights(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, state.Name, svc)
		}
	} else {
		handlerErr = "Not authenticated. Run: advncd login"
	}

	// Observability presets
	logs1h, logsQuery := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "PT1H", "ALL")
	logs6h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "PT6H", "ALL")
	logs24h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "P1D", "ALL")
	logsErr1h, _ := cloudRunLogsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "PT1H", "ERROR")

	metrics1h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "PT1H")
	metrics6h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "PT6H")
	metrics24h := cloudRunMetricsURLWithPreset(cfg.ProjectID, cfg.Region, state.Name, "P1D")

	escapedName := url.PathEscape(state.Name)
	queryErr := strings.TrimSpace(r.URL.Query().Get("error"))
	if queryErr != "" {
		handlerErr = queryErr
	}

	render(views.ServiceDetailVM{
		Error:              handlerErr,
		Name:               state.Name,
		Project:            cfg.ProjectID,
		Region:             cfg.Region,
		SourceType:         strings.TrimSpace(state.SourceType),
		RuntimeStatus:      normalizeStatus(state.Status),
		LastRevision:       strings.TrimSpace(state.LastRevision),
		DeploymentID:       strings.TrimSpace(state.DeploymentID),
		SourcePathOptional: strings.TrimSpace(state.SourcePathOptional),
		PresetIDOptional:   strings.TrimSpace(state.PresetIDOptional),
		URL:                state.URL,
		Image:              image,
		Conditions:         conditions,

		ConsoleURL: cloudRunConsoleURL(cfg.ProjectID, cfg.Region, state.Name),

		LogsURL1h:  logs1h,
		LogsURL6h:  logs6h,
		LogsURL24h: logs24h,
		LogsURLErr: logsErr1h,
		LogsQuery:  logsQuery,

		MetricsURL1h:  metrics1h,
		MetricsURL6h:  metrics6h,
		MetricsURL24h: metrics24h,
		RedeployURL:   "/services/" + escapedName + "/redeploy",
		DeleteURL:     "/services/" + escapedName + "/delete",
		DisableURL:    "/services/" + escapedName + "/disable",

		SafetyProfile:        insights.SafetyProfile,
		SafetyChecks:         mapInsightItems(insights.SafetyChecks),
		SafetyInterpretation: strings.TrimSpace(insights.SafetyInterpretation),
		TrafficItems:         mapInsightItems(insights.Traffic),
		CostItems:            mapInsightItems(insights.Cost),
		Anomalies:            mapInsightBadges(insights.Anomalies),

		Now: time.Now(),

		AuthStatus: authStatus,
		AuthHint:   authHint,
	})
}

func mapInsightItems(items []insightItem) []views.InsightItemVM {
	out := make([]views.InsightItemVM, 0, len(items))
	for _, it := range items {
		out = append(out, views.InsightItemVM{
			Label:  it.Label,
			Value:  it.Value,
			Status: it.Status,
		})
	}
	return out
}

func mapInsightBadges(items []insightBadge) []views.InsightBadgeVM {
	out := make([]views.InsightBadgeVM, 0, len(items))
	for _, it := range items {
		out = append(out, views.InsightBadgeVM{
			Label:    it.Label,
			Severity: it.Severity,
		})
	}
	return out
}
