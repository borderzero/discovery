package discoverers

import (
	"context"
	"sync"
	"time"

	"github.com/borderzero/discovery"
)

// used to set the config of each discoverer e.g. interval
type continuousDiscoveryConfig struct {
	interval   time.Duration
	discoverer discovery.Discoverer
}

// ContinuousDiscoverer represents a discoverer which under-the-hood is multiple
// discoverers and performs continous discovery based on a configured interval.
type ContinuousDiscoverer struct {
	discoverers []continuousDiscoveryConfig
}

// ensure MultipleUpstreamDiscoverer implements discovery.Discoverer at compile-time.
var _ discovery.Discoverer = (*MultipleUpstreamDiscoverer)(nil)

// ContinuousDiscovererOption is an input option for the ContinuousDiscoverer constructor.
type ContinuousDiscovererOption func(*ContinuousDiscoverer)

// WithUpstreamDiscoverers is a configuration option to include additional
// Discoverer(s) in an MultipleUpstreamDiscoverer's discovery.
func WithUpstreamDiscoverer(discoverer discovery.Discoverer, interval time.Duration) ContinuousDiscovererOption {
	return func(cd *ContinuousDiscoverer) {
		cd.discoverers = append(cd.discoverers, continuousDiscoveryConfig{
			discoverer: discoverer,
			interval:   interval,
		})
	}
}

// NewContinuousDiscoverer returns a new ContinuousDiscoverer, initialized with the given options.
func NewContinuousDiscoverer(opts ...ContinuousDiscovererOption) *ContinuousDiscoverer {
	cd := &ContinuousDiscoverer{
		discoverers: []continuousDiscoveryConfig{},
	}
	for _, opt := range opts {
		opt(cd)
	}
	return cd
}

// Discover runs the ContinuousDiscoverer and closes the channels after
// the continuous run of all the underlying discoverers is completed.
// This can only happen if the given context is done.
func (cd *ContinuousDiscoverer) Discover(
	ctx context.Context,
	results chan<- *discovery.Result,
) {
	defer close(results)

	var wg sync.WaitGroup
	for _, discoverer := range cd.discoverers {
		wg.Add(1)

		go runContinuously(
			ctx,
			&wg,
			discoverer.interval,
			discoverer.discoverer,
			results,
		)
	}
	wg.Wait()
}
