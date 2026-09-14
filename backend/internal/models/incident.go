package models

import "time"

type Incident struct {
	ID              string     `json:"id"`
	MonitorID       string     `json:"monitor_id"`
	StartedAt       time.Time  `json:"started_at"`
	RecoveredAt     *time.Time `json:"recovered_at,omitempty"`
	DowntimeSeconds *int64     `json:"downtime_seconds,omitempty"`
	Reason          string     `json:"reason"`
	CreatedAt       time.Time  `json:"created_at"`
}
