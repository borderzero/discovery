package discovery

import (
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
	Errors    []string   `json:"errors"`
	Metadata  Metadata   `json:"metadata"`
}

// NewResult returns a new Result object with
// the StartedAt time set to the current time.
func NewResult(discovererId string) *Result {
	return &Result{
		Resources: []Resource{},
		Errors:    []string{},
		Metadata: Metadata{
			DiscovererId: discovererId,
			StartedAt:    time.Now(),
		},
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
func (r *Result) AddError(err error) {
	r.Lock()
	defer r.Unlock()

	r.Errors = append(r.Errors, err.Error())
}
