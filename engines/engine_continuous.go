package engines

import (
	"context"
	"sync"
	"time"

	"github.com/borderzero/discovery"
)

// discovererConfig represents configuration of each discoverer
type discovererConfig struct {
	interval   time.Duration
	discoverer discovery.Discoverer
}

// ContinuousEngine represents an engine which runs multiple discoverers continuously.
type ContinuousEngine struct {
	discoverers []discovererConfig
}

// ensure MultipleUpstreamDiscoverer implements discovery.Engine at compile-time.
var _ discovery.Engine = (*ContinuousEngine)(nil)

// ContinuousEngineOption is an input option for the ContinuousEngine constructor.
type ContinuousEngineOption func(*ContinuousEngine)

// ContinuousEngineOptionWithDiscoverer is a configuration option to
// include an additional Discoverer in a ContinuousEngine's discovery jobs.
func ContinuousEngineOptionWithDiscoverer(discoverer discovery.Discoverer, interval time.Duration) ContinuousEngineOption {
	return func(engine *ContinuousEngine) {
		engine.discoverers = append(engine.discoverers, discovererConfig{
			interval:   interval,
			discoverer: discoverer,
		})
	}
}

// NewContinuousEngine returns a new ContinuousEngine, initialized with the given options.
func NewContinuousEngine(opts ...ContinuousEngineOption) *ContinuousEngine {
	engine := &ContinuousEngine{discoverers: []discovererConfig{}}
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

// Run runs the ContinuousEngine and closes the results channel after
// the continuous run of all the underlying discoverers is completed.
// ** This will only ever happen when the given context is done **
func (cd *ContinuousEngine) Run(
	ctx context.Context,
	results chan<- *discovery.Result,
) {
	defer close(results)

	var wg sync.WaitGroup
	defer wg.Wait()

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
}
