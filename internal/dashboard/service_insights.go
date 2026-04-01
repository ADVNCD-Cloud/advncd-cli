package dashboard

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpmonitoring"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/safety"
)

type serviceInsights struct {
	SafetyProfile        string
	SafetyChecks         []insightItem
	SafetyInterpretation string
	Traffic              []insightItem
	Cost                 []insightItem
	Anomalies            []insightBadge
}

type insightItem struct {
	Label  string
	Value  string
	Status string // good | caution | danger | info
}

type insightBadge struct {
	Label    string
	Severity string // info | warn | error
}

type actualCostResult struct {
	Available        bool
	AmountEUR        float64
	AsOf             time.Time
	Source           string
	Note             string
	BillingConnected bool
}

type actualCostProvider interface {
	TodayServiceCostEUR(ctx context.Context, projectID, region, serviceName string) (actualCostResult, error)
}

type unavailableActualCostProvider struct{}

func (p unavailableActualCostProvider) TodayServiceCostEUR(ctx context.Context, projectID, region, serviceName string) (actualCostResult, error) {
	_ = ctx
	_ = projectID
	_ = region
	_ = serviceName
	table := strings.TrimSpace(os.Getenv("ADVNCD_BILLING_EXPORT_BQ_TABLE"))
	if table != "" {
		return actualCostResult{
			Available:        false,
			Source:           "billing-export",
			BillingConnected: true,
			Note:             "Billing export is connected. Actual billed cost updates with delay.",
		}, nil
	}
	return actualCostResult{
		Available:        false,
		Source:           "unavailable",
		BillingConnected: false,
		Note:             "Billing export is not connected. Connect export to see actual billed cost.",
	}, nil
}

type estimateResult struct {
	HourlyEUR float64
	DailyEUR  float64
	TodayEUR  float64
	Note      string
}

func buildServiceInsights(
	ctx context.Context,
	accessToken, projectID, region, serviceName string,
	svc *gcprun.ServiceDetail,
) serviceInsights {
	policy := safety.DefaultPolicy(safety.InferProfile(serviceName))

	out := serviceInsights{
		SafetyProfile: policy.DeploymentProfile,
		SafetyChecks:  buildSafetyChecks(svc, policy),
		Cost:          []insightItem{},
		Traffic:       []insightItem{},
		Anomalies:     []insightBadge{},
	}
	out.SafetyInterpretation = deriveSafetyInterpretation(out.SafetyChecks)

	oneHour, err1h := gcpmonitoring.FetchServiceTrafficWindow(ctx, accessToken, projectID, serviceName, time.Hour)
	oneDay, err24h := gcpmonitoring.FetchServiceTrafficWindow(ctx, accessToken, projectID, serviceName, 24*time.Hour)
	if err24h != nil {
		oneDay = nil
	}

	if err1h != nil {
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "traffic",
			Value:  "Cloud Monitoring data unavailable",
			Status: "caution",
		})
	} else {
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "request volume (1h)",
			Value:  fmt.Sprintf("%.0f requests", oneHour.RequestCount),
			Status: "info",
		})
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "requests per minute",
			Value:  fmt.Sprintf("%.2f rpm", oneHour.RequestsPerMinute),
			Status: trafficStatus(oneHour.RequestsPerMinute),
		})
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "latency p95",
			Value:  formatMs(oneHour.LatencyP95Ms),
			Status: "info",
		})
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "error rate",
			Value:  formatRate(oneHour.ErrorRate),
			Status: errorRateStatus(oneHour.ErrorRate),
		})
		out.Traffic = append(out.Traffic, insightItem{
			Label:  "instance activity",
			Value:  formatInstance(oneHour.InstanceAvg),
			Status: "info",
		})
		for _, w := range oneHour.MetricsWarnings {
			out.Traffic = append(out.Traffic, insightItem{
				Label:  "metrics note",
				Value:  w,
				Status: "caution",
			})
		}
	}

	est := estimateCost(svc, oneHour, oneDay)
	out.Cost = append(out.Cost, insightItem{
		Label:  "estimated live cost (hourly)",
		Value:  formatEUR(est.HourlyEUR),
		Status: "info",
	})
	out.Cost = append(out.Cost, insightItem{
		Label:  "estimated live cost (daily)",
		Value:  formatEUR(est.DailyEUR),
		Status: "info",
	})
	out.Cost = append(out.Cost, insightItem{
		Label:  "today estimated cost",
		Value:  formatEUR(est.TodayEUR),
		Status: "info",
	})
	if strings.TrimSpace(est.Note) != "" {
		out.Cost = append(out.Cost, insightItem{
			Label:  "estimate confidence",
			Value:  est.Note,
			Status: "caution",
		})
	}

	provider := unavailableActualCostProvider{}
	actual, err := provider.TodayServiceCostEUR(ctx, projectID, region, serviceName)
	if err != nil {
		out.Cost = append(out.Cost, insightItem{
			Label:  "actual billed cost",
			Value:  "unavailable",
			Status: "caution",
		})
	} else if !actual.Available {
		if actual.BillingConnected {
			out.Cost = append(out.Cost, insightItem{
				Label:  "actual billed cost",
				Value:  "Connected, but billed values update with delay. Use estimates for immediate trend detection.",
				Status: "info",
			})
		} else {
			out.Cost = append(out.Cost, insightItem{
				Label:  "actual billed cost",
				Value:  "Not connected: billing export is not configured yet.",
				Status: "caution",
			})
		}
		if strings.TrimSpace(actual.Note) != "" {
			out.Cost = append(out.Cost, insightItem{
				Label:  "billing note",
				Value:  strings.TrimSpace(actual.Note),
				Status: "info",
			})
		}
	} else {
		out.Cost = append(out.Cost, insightItem{
			Label:  "actual billed cost",
			Value:  formatEUR(actual.AmountEUR),
			Status: "info",
		})
		if !actual.AsOf.IsZero() {
			out.Cost = append(out.Cost, insightItem{
				Label:  "actual as of",
				Value:  actual.AsOf.Local().Format("2006-01-02 15:04"),
				Status: "info",
			})
		}
	}

	out.Anomalies = detectAnomalies(svc, oneHour, oneDay)
	if len(out.Anomalies) == 0 {
		out.Anomalies = append(out.Anomalies, insightBadge{Label: "No active anomaly detected", Severity: "info"})
	}

	return out
}

func buildSafetyChecks(svc *gcprun.ServiceDetail, policy safety.Policy) []insightItem {
	checks := make([]insightItem, 0, 12)
	if svc == nil {
		return checks
	}

	checks = append(checks, insightItem{
		Label:  "deployment profile",
		Value:  policy.DeploymentProfile,
		Status: "info",
	})

	if svc.MinInstances == 0 {
		checks = append(checks, insightItem{
			Label:  "min instances",
			Value:  "0 (scales to zero when idle)",
			Status: "good",
		})
	} else {
		checks = append(checks, insightItem{
			Label:  "min instances",
			Value:  fmt.Sprintf("%d (keeps always-on capacity and baseline cost)", svc.MinInstances),
			Status: "caution",
		})
	}

	if svc.MaxInstances > 0 && svc.MaxInstances <= 1 {
		checks = append(checks, insightItem{
			Label:  "max instances",
			Value:  fmt.Sprintf("%d (tight cap for low-traffic)", svc.MaxInstances),
			Status: "good",
		})
	} else if svc.MaxInstances > 1 {
		checks = append(checks, insightItem{
			Label:  "max instances",
			Value:  fmt.Sprintf("%d (higher scale can increase spend during bursts)", svc.MaxInstances),
			Status: "caution",
		})
	} else {
		checks = append(checks, insightItem{
			Label:  "max instances",
			Value:  "not set",
			Status: "caution",
		})
	}

	if svc.TimeoutSeconds > 0 && svc.TimeoutSeconds <= 30 {
		checks = append(checks, insightItem{
			Label:  "request timeout",
			Value:  fmt.Sprintf("%ds (tight for low-cost profile)", svc.TimeoutSeconds),
			Status: "good",
		})
	} else if svc.TimeoutSeconds > 0 {
		checks = append(checks, insightItem{
			Label:  "request timeout",
			Value:  fmt.Sprintf("%ds (long timeouts increase risk and cost)", svc.TimeoutSeconds),
			Status: "caution",
		})
	} else {
		checks = append(checks, insightItem{
			Label:  "request timeout",
			Value:  "not reported by API",
			Status: "caution",
		})
	}

	memMi := parseMemoryMi(svc.Memory)
	if memMi > 0 && memMi <= 512 {
		checks = append(checks, insightItem{Label: "runtime memory", Value: fmt.Sprintf("%dMi", memMi), Status: "good"})
	} else if memMi > 0 {
		checks = append(checks, insightItem{Label: "runtime memory", Value: fmt.Sprintf("%dMi (higher than typical low-traffic baseline)", memMi), Status: "caution"})
	} else {
		checks = append(checks, insightItem{Label: "runtime memory", Value: "not reported by API", Status: "caution"})
	}

	secretRefs := len(svc.SecretEnvKeys)
	plainSensitive := countPlainSensitiveEnv(svc.Env)
	if secretRefs > 0 {
		checks = append(checks, insightItem{
			Label:  "secret manager references",
			Value:  fmt.Sprintf("%d detected", secretRefs),
			Status: "good",
		})
	} else if policy.DeploymentProfile == safety.ProfileWebhookLowTraffic {
		checks = append(checks, insightItem{
			Label:  "secret manager references",
			Value:  "none detected for webhook profile",
			Status: "caution",
		})
	} else {
		checks = append(checks, insightItem{
			Label:  "secret manager references",
			Value:  "none detected",
			Status: "info",
		})
	}

	checks = append(checks, insightItem{
		Label:  "plain sensitive env",
		Value:  "none detected",
		Status: "good",
	})
	if plainSensitive > 0 {
		checks[len(checks)-1] = insightItem{
			Label:  "plain sensitive env",
			Value:  fmt.Sprintf("%d key(s) detected in plain env", plainSensitive),
			Status: "danger",
		}
	}

	querySecretCount := countQuerySecretEnvValues(svc.Env)
	if querySecretCount == 0 {
		checks = append(checks, insightItem{
			Label:  "query-string secrets",
			Value:  "none detected",
			Status: "good",
		})
	} else {
		checks = append(checks, insightItem{
			Label:  "query-string secrets",
			Value:  fmt.Sprintf("%d insecure value(s) detected", querySecretCount),
			Status: "danger",
		})
	}

	if policy.DeploymentProfile == safety.ProfileWebhookLowTraffic {
		if secretRefs == 0 && plainSensitive == 0 {
			checks = append(checks, insightItem{
				Label:  "webhook protection signal",
				Value:  "No auth secret detected. Public webhook exposure risk.",
				Status: "danger",
			})
		} else {
			checks = append(checks, insightItem{
				Label:  "webhook protection signal",
				Value:  "Auth secret detected for webhook profile.",
				Status: "good",
			})
		}
	}

	return checks
}

func deriveSafetyInterpretation(checks []insightItem) string {
	hasDanger := false
	hasCaution := false
	for _, c := range checks {
		switch c.Status {
		case "danger":
			hasDanger = true
		case "caution":
			hasCaution = true
		}
	}
	if hasDanger {
		return "Danger items need immediate action. Resolve security risks first, then optimize cost."
	}
	if hasCaution {
		return "Service is running, but caution items can increase cost or reduce reliability."
	}
	return "Configuration aligns with the recommended safety baseline."
}

func countPlainSensitiveEnv(env map[string]string) int {
	n := 0
	for k, v := range env {
		if !safety.IsSensitiveKey(k) {
			continue
		}
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

func countQuerySecretEnvValues(env map[string]string) int {
	n := 0
	for _, v := range env {
		if safety.LooksLikeQuerySecret(v) {
			n++
		}
	}
	return n
}

func estimateCost(svc *gcprun.ServiceDetail, oneHour, oneDay *gcpmonitoring.ServiceTrafficWindow) estimateResult {
	out := estimateResult{}
	if oneHour == nil {
		out.Note = "No recent traffic metrics were available, so cost estimate confidence is low."
		return out
	}

	// Rough heuristics for Cloud Run on-demand pricing (EUR approximation).
	const (
		eurPerVCPUSecond = 0.000024 * 0.92
		eurPerGiBSecond  = 0.0000025 * 0.92
		eurPerMillionReq = 0.40 * 0.92
		defaultLatencyS  = 0.2
		defaultVCPU      = 1.0
	)

	memGiB := parseMemoryGiB(svcMemory(svc))
	if memGiB <= 0 {
		memGiB = 0.25
	}
	avgLatencyS := defaultLatencyS
	note := ""
	if oneHour.LatencyAvgMs != nil && *oneHour.LatencyAvgMs > 0 {
		avgLatencyS = *oneHour.LatencyAvgMs / 1000.0
		note = "Estimated from recent request and runtime activity. This is directional and not the billed amount."
	} else {
		note = "Estimated from request volume with fallback response-time assumptions. Useful for trend monitoring, but less accurate than live runtime metrics."
	}

	requestCostHour := (oneHour.RequestCount / 1_000_000.0) * eurPerMillionReq
	activeSecondsHour := oneHour.RequestCount * avgLatencyS
	cpuCostHour := activeSecondsHour * defaultVCPU * eurPerVCPUSecond
	memCostHour := activeSecondsHour * memGiB * eurPerGiBSecond
	out.HourlyEUR = requestCostHour + cpuCostHour + memCostHour
	out.DailyEUR = out.HourlyEUR * 24.0

	if oneDay != nil {
		requestCostToday := (oneDay.RequestCount / 1_000_000.0) * eurPerMillionReq
		activeSecondsToday := oneDay.RequestCount * avgLatencyS
		cpuCostToday := activeSecondsToday * defaultVCPU * eurPerVCPUSecond
		memCostToday := activeSecondsToday * memGiB * eurPerGiBSecond
		out.TodayEUR = requestCostToday + cpuCostToday + memCostToday
	} else {
		out.TodayEUR = out.DailyEUR
		if note != "" {
			note += "; "
		}
		note += "Today's estimate is extrapolated from recent activity."
	}

	out.Note = note
	return out
}

func detectAnomalies(svc *gcprun.ServiceDetail, oneHour, oneDay *gcpmonitoring.ServiceTrafficWindow) []insightBadge {
	out := []insightBadge{}
	if oneHour == nil {
		return out
	}

	baselineRPM := 0.0
	if oneDay != nil {
		baselineRPM = oneDay.RequestsPerMinute
	}

	if oneHour.RequestsPerMinute >= math.Max(5.0, baselineRPM*3.0) && oneHour.RequestCount >= 100 {
		out = append(out, insightBadge{Label: "Request spike vs baseline", Severity: "warn"})
	}

	if oneHour.ErrorRate != nil && *oneHour.ErrorRate >= 0.05 && oneHour.RequestCount >= 20 {
		out = append(out, insightBadge{Label: "Elevated error rate", Severity: "error"})
	}

	if svc != nil && svc.MinInstances == 0 && oneHour.RequestsPerMinute > 0.3 {
		out = append(out, insightBadge{Label: "Service active while min-instances=0", Severity: "info"})
	}

	if svc != nil && svc.MaxInstances > 0 && svc.MaxInstances <= 1 && oneHour.RequestsPerMinute > 60 {
		out = append(out, insightBadge{Label: "Single-instance saturation risk", Severity: "warn"})
	}

	return out
}

func svcMemory(svc *gcprun.ServiceDetail) string {
	if svc == nil {
		return ""
	}
	return strings.TrimSpace(svc.Memory)
}

func parseMemoryMi(raw string) int {
	v := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasSuffix(v, "mi") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(v, "mi")))
		if err == nil && n > 0 {
			return n
		}
	}
	if strings.HasSuffix(v, "gi") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(v, "gi")))
		if err == nil && n > 0 {
			return n * 1024
		}
	}
	return 0
}

func parseMemoryGiB(raw string) float64 {
	mi := parseMemoryMi(raw)
	if mi <= 0 {
		return 0
	}
	return float64(mi) / 1024.0
}

func formatEUR(v float64) string {
	return fmt.Sprintf("~€%.4f", v)
}

func formatMs(v *float64) string {
	if v == nil {
		return "unavailable"
	}
	return fmt.Sprintf("%.2f ms", *v)
}

func formatRate(v *float64) string {
	if v == nil {
		return "unavailable"
	}
	return fmt.Sprintf("%.2f%%", *v*100.0)
}

func formatInstance(v *float64) string {
	if v == nil {
		return "unavailable"
	}
	return fmt.Sprintf("avg %.2f active instances", *v)
}

func trafficStatus(rpm float64) string {
	if rpm >= 60 {
		return "caution"
	}
	if rpm > 0 {
		return "info"
	}
	return "good"
}

func errorRateStatus(rate *float64) string {
	if rate == nil {
		return "caution"
	}
	if *rate >= 0.05 {
		return "danger"
	}
	if *rate > 0 {
		return "caution"
	}
	return "good"
}
