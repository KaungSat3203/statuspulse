package models

import "time"

type CheckResult struct {
	MonitorID  string    `json:"monitor_id"`
	CheckedAt  time.Time `json:"checked_at"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	Reachable  bool      `json:"reachable"`
	Error      string    `json:"error"`
}
