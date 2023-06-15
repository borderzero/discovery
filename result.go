package discovery

import (
	"sync"
	"time"
)

// Metrics represents stats/metrics for a
type Metrics struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Result represents the result of a discoverer
type Result struct {
	sync.Mutex // inherit lock behaviour

	Resources []Resource `json:"resources"`
	Errors    []string   `json:"errors"`
	Metrics   Metrics    `json:"metrics"`
}

// NewResult returns a new Result object with
// the StartedAt time set to the current time.
func NewResult() *Result {
	return &Result{
		Resources: []Resource{},
		Errors:    []string{},
		Metrics: Metrics{
			StartedAt: time.Now(),
		},
	}
}

// Done sets the EndedAt time in a Result to the current time.
func (r *Result) Done() {
	r.Lock()
	defer r.Unlock()

	r.Metrics.EndedAt = time.Now()
}

// AddResource adds a resource to a result
func (r *Result) AddResource(resource Resource) {
	r.Lock()
	defer r.Unlock()

	r.Resources = append(r.Resources, resource)
}

// AddError adds an error to a result
func (r *Result) AddError(err error) {
	r.Lock()
	defer r.Unlock()

	r.Errors = append(r.Errors, err.Error())
}
