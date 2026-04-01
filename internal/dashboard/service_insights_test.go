package dashboard

import (
	"testing"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpmonitoring"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

func TestParseMemoryMi(t *testing.T) {
	t.Parallel()

	if got := parseMemoryMi("256Mi"); got != 256 {
		t.Fatalf("expected 256, got %d", got)
	}
	if got := parseMemoryMi("1Gi"); got != 1024 {
		t.Fatalf("expected 1024, got %d", got)
	}
	if got := parseMemoryMi("bad"); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestEstimateCostWithFallbackLatency(t *testing.T) {
	t.Parallel()

	oneHour := &gcpmonitoring.ServiceTrafficWindow{RequestCount: 1200}
	oneDay := &gcpmonitoring.ServiceTrafficWindow{RequestCount: 24000}
	est := estimateCost(&gcprun.ServiceDetail{Memory: "256Mi"}, oneHour, oneDay)

	if est.HourlyEUR <= 0 {
		t.Fatalf("expected hourly estimate > 0, got %f", est.HourlyEUR)
	}
	if est.TodayEUR <= 0 {
		t.Fatalf("expected today estimate > 0, got %f", est.TodayEUR)
	}
	if est.Note == "" {
		t.Fatal("expected fallback note when avg latency is missing")
	}
}

func TestDetectAnomaliesSpikeAndError(t *testing.T) {
	t.Parallel()

	errRate := 0.12
	oneHour := &gcpmonitoring.ServiceTrafficWindow{
		Start:             time.Now().Add(-time.Hour),
		End:               time.Now(),
		RequestCount:      500,
		RequestsPerMinute: 100,
		ErrorRate:         &errRate,
	}
	oneDay := &gcpmonitoring.ServiceTrafficWindow{RequestsPerMinute: 10}
	svc := &gcprun.ServiceDetail{MinInstances: 0, MaxInstances: 1}
	anomalies := detectAnomalies(svc, oneHour, oneDay)

	if len(anomalies) == 0 {
		t.Fatal("expected anomalies, got none")
	}
}
