package discovery

import "time"

// Metrics represents stats/metrics for a
type Metrics struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Result represents the result of a discoverer
type Result struct {
	Resources []Resource `json:"resources"`
	Errors    []error    `json:"errors"`
	Metrics   Metrics    `json:"metrics"`
}

// NewResult returns a newly intialized Result object
func NewResult() *Result {
	return &Result{
		Resources: []Resource{},
		Errors:    []error{},
		Metrics: Metrics{
			StartedAt: time.Now(),
		},
	}
}
