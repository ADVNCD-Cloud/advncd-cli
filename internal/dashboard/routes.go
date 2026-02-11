package dashboard

import (
	"net/http"
	"strings"
)

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", overviewHandler)
	mux.HandleFunc("/services", servicesListHandler)

	// One predictable handler for everything under /services/
	mux.HandleFunc("/services/", servicesSubrouterHandler)

	// Static files (quick mode; depends on cwd)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("internal/dashboard/static"))))
}

func servicesSubrouterHandler(w http.ResponseWriter, r *http.Request) {
	// Supported:
	// - /services/<name...>
	// - /services/<name...>/explain
	// - /services/<name...>/explain/stream

	rel := strings.TrimPrefix(r.URL.Path, "/services/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(rel, "/explain/stream") {
		serviceExplainStreamHandler(w, r)
		return
	}

	if strings.HasSuffix(rel, "/explain") {
		serviceExplainHandler(w, r)
		return
	}

	serviceDetailHandler(w, r)
}

func serviceNameFromPath(r *http.Request) (string, bool) {
	rel := strings.TrimPrefix(r.URL.Path, "/services/")
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return "", false
	}

	// Strip action suffixes
	if strings.HasSuffix(rel, "/explain/stream") {
		rel = strings.TrimSuffix(rel, "/explain/stream")
		rel = strings.Trim(rel, "/")
	} else if strings.HasSuffix(rel, "/explain") {
		rel = strings.TrimSuffix(rel, "/explain")
		rel = strings.Trim(rel, "/")
	}

	if rel == "" {
		return "", false
	}

	// IMPORTANT: allow '/' inside name (multi-segment)
	return rel, true
}