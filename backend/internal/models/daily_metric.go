package models

import "time"

type DailyMetric struct {
	ID               int64     `json:"id"`
	MonitorID        string    `json:"monitor_id"`
	MetricDate       time.Time `json:"metric_date"`
	TotalChecks      int64     `json:"total_checks"`
	SuccessfulChecks int64     `json:"successful_checks"`
	MeanLatencyMs    *float64  `json:"mean_latency_ms,omitempty"`
	P95LatencyMs     *float64  `json:"p95_latency_ms,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
