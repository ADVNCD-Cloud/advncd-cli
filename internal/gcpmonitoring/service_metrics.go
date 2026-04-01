package gcpmonitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrMetricsRead = apperr.E("M-MON-001", "Failed to read Cloud Monitoring metrics")

type ServiceTrafficWindow struct {
	Start time.Time
	End   time.Time

	RequestCount      float64
	RequestsPerMinute float64
	LatencyP95Ms      *float64
	LatencyAvgMs      *float64
	ErrorCount        *float64
	ErrorRate         *float64
	InstanceAvg       *float64

	MetricsWarnings []string
}

func FetchServiceTrafficWindow(ctx context.Context, accessToken, projectID, serviceName string, window time.Duration) (*ServiceTrafficWindow, error) {
	if window <= 0 {
		window = time.Hour
	}
	end := time.Now().UTC()
	start := end.Add(-window)

	out := &ServiceTrafficWindow{
		Start: start,
		End:   end,
	}

	totalReqValues, err := queryMetricValues(ctx, accessToken, projectID, buildFilter(serviceName, `metric.type="run.googleapis.com/request_count"`), start, end, "ALIGN_SUM", "REDUCE_SUM", "60s")
	if err != nil {
		return nil, err
	}
	out.RequestCount = sum(totalReqValues)
	if mins := window.Minutes(); mins > 0 {
		out.RequestsPerMinute = out.RequestCount / mins
	}

	// Best-effort 5xx filter; keep this non-fatal if metric label schema differs.
	errVals, err5xx := queryMetricValues(ctx, accessToken, projectID, buildFilter(serviceName, `metric.type="run.googleapis.com/request_count" AND metric.labels.response_code_class="5xx"`), start, end, "ALIGN_SUM", "REDUCE_SUM", "60s")
	if err5xx == nil {
		errCount := sum(errVals)
		out.ErrorCount = &errCount
		if out.RequestCount > 0 {
			errRate := errCount / out.RequestCount
			out.ErrorRate = &errRate
		}
	} else {
		out.MetricsWarnings = append(out.MetricsWarnings, "5xx error metric unavailable")
	}

	p95Vals, err := queryMetricValues(ctx, accessToken, projectID, buildFilter(serviceName, `metric.type="run.googleapis.com/request_latencies"`), start, end, "ALIGN_PERCENTILE_95", "REDUCE_MEAN", "60s")
	if err == nil && len(p95Vals) > 0 {
		v := avg(p95Vals) * 1000.0
		out.LatencyP95Ms = &v
	} else {
		out.MetricsWarnings = append(out.MetricsWarnings, "p95 latency metric unavailable")
	}

	avgVals, err := queryMetricValues(ctx, accessToken, projectID, buildFilter(serviceName, `metric.type="run.googleapis.com/request_latencies"`), start, end, "ALIGN_MEAN", "REDUCE_MEAN", "60s")
	if err == nil && len(avgVals) > 0 {
		v := avg(avgVals) * 1000.0
		out.LatencyAvgMs = &v
	} else {
		out.MetricsWarnings = append(out.MetricsWarnings, "average latency metric unavailable")
	}

	instanceVals, err := queryMetricValues(ctx, accessToken, projectID, buildFilter(serviceName, `metric.type="run.googleapis.com/container/instance_count"`), start, end, "ALIGN_MEAN", "REDUCE_MEAN", "60s")
	if err == nil && len(instanceVals) > 0 {
		v := avg(instanceVals)
		out.InstanceAvg = &v
	} else {
		out.MetricsWarnings = append(out.MetricsWarnings, "instance activity metric unavailable")
	}

	return out, nil
}

func buildFilter(serviceName string, metricExpr string) string {
	return strings.Join([]string{
		`resource.type="cloud_run_revision"`,
		`resource.labels.service_name="` + strings.TrimSpace(serviceName) + `"`,
		metricExpr,
	}, " AND ")
}

type timeSeriesListResponse struct {
	TimeSeries []struct {
		Points []struct {
			Value struct {
				DoubleValue       *float64 `json:"doubleValue,omitempty"`
				Int64Value        string   `json:"int64Value,omitempty"`
				DistributionValue struct {
					Mean float64 `json:"mean,omitempty"`
				} `json:"distributionValue,omitempty"`
			} `json:"value"`
		} `json:"points"`
	} `json:"timeSeries"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

func queryMetricValues(ctx context.Context, accessToken, projectID, filter string, start, end time.Time, aligner, reducer, alignmentPeriod string) ([]float64, error) {
	base := "https://monitoring.googleapis.com/v3/projects/" + url.PathEscape(strings.TrimSpace(projectID)) + "/timeSeries"
	client := &http.Client{Timeout: 25 * time.Second}

	values := make([]float64, 0, 64)
	pageToken := ""

	for {
		u, _ := url.Parse(base)
		q := u.Query()
		q.Set("filter", filter)
		q.Set("interval.startTime", start.Format(time.RFC3339))
		q.Set("interval.endTime", end.Format(time.RFC3339))
		q.Set("aggregation.alignmentPeriod", alignmentPeriod)
		q.Set("aggregation.perSeriesAligner", aligner)
		if strings.TrimSpace(reducer) != "" {
			q.Set("aggregation.crossSeriesReducer", reducer)
		}
		q.Set("view", "FULL")
		q.Set("pageSize", "1000")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, apperr.New(ErrMetricsRead).WithCause(err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := client.Do(req)
		if err != nil {
			return nil, apperr.New(ErrMetricsRead).WithCause(err)
		}
		raw, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, apperr.New(ErrMetricsRead).
				WithMeta("http_status", res.Status).
				WithMeta("raw_body", string(raw)).
				WithMeta("filter", filter)
		}

		var out timeSeriesListResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, apperr.New(ErrMetricsRead).WithCause(err).
				WithMeta("raw_body", string(raw))
		}

		for _, ts := range out.TimeSeries {
			for _, p := range ts.Points {
				if p.Value.DoubleValue != nil {
					values = append(values, *p.Value.DoubleValue)
					continue
				}
				if strings.TrimSpace(p.Value.Int64Value) != "" {
					if iv, err := strconv.ParseFloat(strings.TrimSpace(p.Value.Int64Value), 64); err == nil {
						values = append(values, iv)
						continue
					}
				}
				if p.Value.DistributionValue.Mean != 0 {
					values = append(values, p.Value.DistributionValue.Mean)
				}
			}
		}

		if strings.TrimSpace(out.NextPageToken) == "" {
			break
		}
		pageToken = out.NextPageToken
	}

	return values, nil
}

func sum(v []float64) float64 {
	var out float64
	for _, x := range v {
		out += x
	}
	return out
}

func avg(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return sum(v) / float64(len(v))
}

func FormatMetricFloat(v float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, v)
}
