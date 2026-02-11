package dashboard

import (
	"net/url"
	"strings"
)

func cloudRunConsoleURL(projectID, region, serviceName string) string {
	return "https://console.cloud.google.com/run/detail/" + region + "/" + serviceName + "?project=" + projectID
}

// timeRange examples: "PT1H", "PT6H", "P1D" (ISO-8601 durations)
func cloudRunLogsURL(projectID, region, serviceName, timeRange, severity string) (string, string) {
	// Returns (url, query) so we can also "Copy query"
	query := `resource.type="cloud_run_revision"
resource.labels.service_name="` + serviceName + `"
resource.labels.location="` + region + `"`

	if strings.TrimSpace(severity) != "" && severity != "ALL" {
		// Cloud Logging uses severity>=ERROR, etc. We'll keep it simple:
		// "ERROR" preset will show only ERROR+
		query += "\nseverity>=" + severity
	}

	u := "https://console.cloud.google.com/logs/query;query=" + url.QueryEscape(query)

	// timeRange in console is not perfectly stable, but this works well enough:
	// We'll use "duration" query param if present; otherwise users can adjust in UI.
	// If it doesn't apply in some accounts, it's harmless.
	if strings.TrimSpace(timeRange) != "" {
		u += ";duration=" + url.QueryEscape(timeRange)
	}

	u += "?project=" + url.QueryEscape(projectID)
	return u, query
}

func cloudRunMetricsURL(projectID, region, serviceName, timeRange string) string {
	u := "https://console.cloud.google.com/monitoring/metrics-explorer" +
		"?project=" + url.QueryEscape(projectID) +
		"&resource=cloud_run_revision" +
		"&resource.label.service_name=" + url.QueryEscape(serviceName) +
		"&resource.label.location=" + url.QueryEscape(region)

	// duration hint (may be ignored by UI depending on console changes, safe to include)
	if strings.TrimSpace(timeRange) != "" {
		u += "&duration=" + url.QueryEscape(timeRange)
	}
	return u
}