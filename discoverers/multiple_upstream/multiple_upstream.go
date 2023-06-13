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
	engine := &MultipleUpstreamDiscoverer{
		discoverers: []discovery.Discoverer{},
	}
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

// Discover runs the MultipleUpstreamDiscoverer and closes the given resources
// channel after a single run of all the underlying discoverers is completed.
func (mud *MultipleUpstreamDiscoverer) Discover(ctx context.Context, resources chan<- []discovery.Resource) {
	var wg sync.WaitGroup
	for _, discoverer := range mud.discoverers {
		wg.Add(1)
		go func(d discovery.Discoverer) {
			defer wg.Done()
			d.Discover(ctx, resources)
		}(discoverer)
	}
	wg.Wait()
	close(resources)
}
