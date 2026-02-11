package dashboard

import (
	"net/url"
	"strings"
)

// timeRange examples: "PT1H", "PT6H", "P1D" (ISO-8601 durations)
func cloudRunLogsURLWithPreset(projectID, region, serviceName, timeRange, severity string) (string, string) {
	query := `resource.type="cloud_run_revision"
resource.labels.service_name="` + serviceName + `"
resource.labels.location="` + region + `"`

	if strings.TrimSpace(severity) != "" && severity != "ALL" {
		query += "\nseverity>=" + severity
	}

	u := "https://console.cloud.google.com/logs/query;query=" + url.QueryEscape(query)

	if strings.TrimSpace(timeRange) != "" {
		u += ";duration=" + url.QueryEscape(timeRange)
	}

	u += "?project=" + url.QueryEscape(projectID)
	return u, query
}

func cloudRunMetricsURLWithPreset(projectID, region, serviceName, timeRange string) string {
	u := "https://console.cloud.google.com/monitoring/metrics-explorer" +
		"?project=" + url.QueryEscape(projectID) +
		"&resource=cloud_run_revision" +
		"&resource.label.service_name=" + url.QueryEscape(serviceName) +
		"&resource.label.location=" + url.QueryEscape(region)

	if strings.TrimSpace(timeRange) != "" {
		u += "&duration=" + url.QueryEscape(timeRange)
	}
	return u
}