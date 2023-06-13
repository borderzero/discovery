package discoverers

import (
	"context"
	"sync"

	"github.com/borderzero/discovery"
	"github.com/borderzero/discovery/discoverers/utils"
)

// MultipleUpstreamDiscoverer represents a discoverer which under-the-hood is multiple discoverers.
type MultipleUpstreamDiscoverer struct {
	discoverers []discovery.Discoverer
}

// ensure MultipleUpstreamDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*MultipleUpstreamDiscoverer)(nil)

// MultipleUpstreamOption is an input option for the MultipleUpstreamDiscoverer constructor.
type MultipleUpstreamOption func(*MultipleUpstreamDiscoverer)

// WithUpstreamDiscoverers is a configuration option to include additional
// Discoverer(s) in an MultipleUpstreamDiscoverer's discovery.
func WithUpstreamDiscoverers(discoverers ...discovery.Discoverer) MultipleUpstreamOption {
	return func(mud *MultipleUpstreamDiscoverer) {
		mud.discoverers = append(mud.discoverers, discoverers...)
	}
}

// NewMultipleUpstreamDiscoverer returns a new MultipleUpstreamDiscoverer, initialized with the given options.
func NewMultipleUpstreamDiscoverer(opts ...MultipleUpstreamOption) *MultipleUpstreamDiscoverer {
	mud := &MultipleUpstreamDiscoverer{
		discoverers: []discovery.Discoverer{},
	}
	for _, opt := range opts {
		opt(mud)
	}
	return mud
}

// Discover runs the MultipleUpstreamDiscoverer and closes the channels
// after a single run of all the underlying discoverers is completed.
func (mud *MultipleUpstreamDiscoverer) Discover(
	ctx context.Context,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	// discover routines are in charge of
	// closing their channels when done
	defer func() {
		close(resources)
		close(errors)
	}()

	var wg sync.WaitGroup
	for _, discoverer := range mud.discoverers {
		wg.Add(1)

		go utils.RunOneOff(
			ctx,
			&wg,
			discoverer,
			resources,
			errors,
		)
	}
	wg.Wait()
}
