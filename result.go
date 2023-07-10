package discovery

import (
	"fmt"
	"sync"
	"time"
)

// Metadata represents metadata for a result.
type Metadata struct {
	DiscovererId string    `json:"discoverer_id"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at"`
}

// Result represents the result of a discoverer.
type Result struct {
	sync.Mutex // inherit lock behaviour

	Resources []Resource `json:"resources"`
	Metadata  Metadata   `json:"metadata"`

	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// NewResult returns a new Result object with
// the StartedAt time set to the current time.
func NewResult(discovererId string) *Result {
	return &Result{
		Resources: []Resource{},
		Metadata: Metadata{
			DiscovererId: discovererId,
			StartedAt:    time.Now(),
		},
		Errors:   []string{},
		Warnings: []string{},
	}
}

// Done sets the EndedAt time in a Result to the current time.
func (r *Result) Done() {
	r.Lock()
	defer r.Unlock()

	r.Metadata.EndedAt = time.Now()
}

// AddResources adds resources to a result
func (r *Result) AddResources(resources ...Resource) {
	r.Lock()
	defer r.Unlock()

	r.Resources = append(r.Resources, resources...)
}

// AddError adds an error to a result
func (r *Result) AddError(err string) {
	r.Lock()
	defer r.Unlock()

	r.Errors = append(r.Errors, err)
}

// AddErrorf adds a formatted error to a result
func (r *Result) AddErrorf(template string, args ...any) {
	r.AddError(fmt.Sprintf(template, args...))
}

// AddWarning adds an warning to a result
func (r *Result) AddWarning(warn string) {
	r.Lock()
	defer r.Unlock()

	r.Warnings = append(r.Warnings, warn)
}

// AddWarningf adds a formatted warning to a result
func (r *Result) AddWarningf(template string, args ...any) {
	r.AddWarning(fmt.Sprintf(template, args...))
}
