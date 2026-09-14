package models

import "time"

type Monitor struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	URL                  string    `json:"url"`
	ExpectedStatusCode   int       `json:"expected_status_code"`
	IntervalSeconds      int       `json:"interval_seconds"`
	TimeoutSeconds       int       `json:"timeout_seconds"`
	Active               bool      `json:"active"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type MonitorUpdate struct {
	Name               *string
	URL                *string
	ExpectedStatusCode *int
	IntervalSeconds    *int
	TimeoutSeconds     *int
	Active             *bool
}

func (u MonitorUpdate) HasChanges() bool {
	return u.Name != nil ||
		u.URL != nil ||
		u.ExpectedStatusCode != nil ||
		u.IntervalSeconds != nil ||
		u.TimeoutSeconds != nil ||
		u.Active != nil
}
