package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func serviceRedeployHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, ok := serviceNameFromPath(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	returnTo := actionReturnTo(r, "/services/"+url.PathEscape(name))

	cfgStore, err := config.DefaultStore()
	if err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" || strings.TrimSpace(cfg.Region) == "" {
		redirectWithError(w, r, returnTo, "Project not initialized. Run: advncd init")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		redirectWithError(w, r, returnTo, "Not authenticated. Run: advncd login")
		return
	}

	if err := gcprun.RedeployService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name); err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}

	state := serviceState{
		Name:      name,
		ProjectID: cfg.ProjectID,
		Region:    cfg.Region,
		Status:    "unknown",
		UpdatedAt: time.Now().UTC(),
	}

	if regState, loadErr := loadRegistryServiceState(cfg.ProjectID, cfg.Region); loadErr == nil {
		if prev, exists := regState[name]; exists {
			state.SourceType = prev.SourceType
			state.DeploymentID = prev.DeploymentID
			state.SourcePathOptional = prev.SourcePathOptional
			state.PresetIDOptional = prev.PresetIDOptional
		}
	}

	if svc, svcErr := gcprun.GetService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name); svcErr == nil && svc != nil {
		if strings.TrimSpace(svc.URL) != "" {
			state.URL = strings.TrimSpace(svc.URL)
		}
		state.Status = normalizeStatus(svc.Status)
		state.LastRevision = strings.TrimSpace(svc.LatestRevision)
	}

	if err := upsertServiceStateToRegistry(state); err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func serviceDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, ok := serviceNameFromPath(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	returnTo := actionReturnTo(r, "/services")

	cfgStore, err := config.DefaultStore()
	if err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" || strings.TrimSpace(cfg.Region) == "" {
		redirectWithError(w, r, returnTo, "Project not initialized. Run: advncd init")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		redirectWithError(w, r, returnTo, "Not authenticated. Run: advncd login")
		return
	}

	if err := gcprun.DeleteService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name); err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}
	if err := removeServiceStateFromRegistry(name, cfg.ProjectID, cfg.Region); err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func serviceDisableHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, ok := serviceNameFromPath(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	returnTo := actionReturnTo(r, "/services/"+url.PathEscape(name))

	cfgStore, err := config.DefaultStore()
	if err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" || strings.TrimSpace(cfg.Region) == "" {
		redirectWithError(w, r, returnTo, "Project not initialized. Run: advncd init")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		redirectWithError(w, r, returnTo, "Not authenticated. Run: advncd login")
		return
	}

	if err := gcprun.DenyUnauthenticated(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name); err != nil {
		redirectWithError(w, r, returnTo, err.Error())
		return
	}

	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func actionReturnTo(r *http.Request, fallback string) string {
	if err := r.ParseForm(); err != nil {
		return fallback
	}
	v := strings.TrimSpace(r.FormValue("return_to"))
	if !strings.HasPrefix(v, "/") {
		return fallback
	}
	return v
}

func redirectWithError(w http.ResponseWriter, r *http.Request, target, msg string) {
	if strings.TrimSpace(msg) == "" {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	u, err := url.Parse(target)
	if err != nil {
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	q := u.Query()
	q.Set("error", msg)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}
