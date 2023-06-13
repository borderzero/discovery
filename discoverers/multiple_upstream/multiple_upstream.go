package multiple_upstream

import (
	"context"
	"sync"

	"github.com/borderzero/discovery"
)

// MultipleUpstreamDiscoverer represents a discoverer which under-the-hood is multiple discoverers.
type MultipleUpstreamDiscoverer struct {
	discoverers []discovery.Discoverer
}

// ensure MultipleUpstreamDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*MultipleUpstreamDiscoverer)(nil)

// Option is an input option for the MultipleUpstreamDiscoverer constructor.
type Option func(*MultipleUpstreamDiscoverer)

// WithDiscoverers is a configuration option to include Discoverer(s) in an Engine's discovery.
func WithDiscoverers(discoverers ...discovery.Discoverer) Option {
	return func(mud *MultipleUpstreamDiscoverer) {
		mud.discoverers = append(mud.discoverers, discoverers...)
	}
}

// NewEngine returns a new engine, initialized with the given options.
func NewMultipleUpstreamDiscoverer(opts ...Option) *MultipleUpstreamDiscoverer {
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
	var wg sync.WaitGroup
	for _, discoverer := range mud.discoverers {
		wg.Add(1)
		go runOne(ctx, discoverer, &wg, resources, errors)
	}
	wg.Wait()

	close(resources)
	close(errors)
}

// runOne runs a discoverer and signals a wait group when done.
func runOne(
	ctx context.Context,
	discoverer discovery.Discoverer,
	wg *sync.WaitGroup,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	defer wg.Done()
	discoverer.Discover(ctx, resources, errors)
}
