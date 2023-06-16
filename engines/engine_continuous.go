package engines

import (
	"context"
	"sync"
	"time"

	"github.com/borderzero/discovery"
)

const (
	defaultInterval = time.Minute * 5
)

type discovererConfig struct {
	interval  time.Duration
	intervalC <-chan time.Duration
	triggerC  <-chan struct{}

	discoverer discovery.Discoverer
}

// ContinuousEngine represents an engine which runs multiple discoverers continuously.
type ContinuousEngine struct {
	discoverers []*discovererConfig
}

// ensure MultipleUpstreamDiscoverer implements discovery.Engine at compile-time.
var _ discovery.Engine = (*ContinuousEngine)(nil)

// ContinuousEngineOption is an input option for the ContinuousEngine constructor.
type ContinuousEngineOption func(*ContinuousEngine)

// DiscovererOption is an input option for ContinuousEngine's WithDiscoverer().
type DiscovererOption func(*discovererConfig)

// WithInitialInterval sets a non-default initial interval for a ContinuousEngine's discoverer.
func WithInitialInterval(interval time.Duration) DiscovererOption {
	return func(dc *discovererConfig) {
		dc.interval = interval
	}
}

// WithIntervalChannel sets the interval (updates) channel for a ContinuousEngine's discoverer.
func WithIntervalChannel(intervalC <-chan time.Duration) DiscovererOption {
	return func(dc *discovererConfig) {
		dc.intervalC = intervalC
	}
}

// WithTriggerChannel sets the trigger (manual triggers) channel for a ContinuousEngine's discoverer.
func WithTriggerChannel(triggerC <-chan struct{}) DiscovererOption {
	return func(dc *discovererConfig) {
		dc.triggerC = triggerC
	}
}

// WithDiscoverer is a configuration option to include an
// additional Discoverer in a ContinuousEngine's discovery jobs.
func WithDiscoverer(discoverer discovery.Discoverer, opts ...DiscovererOption) ContinuousEngineOption {
	return func(engine *ContinuousEngine) {
		dc := &discovererConfig{
			discoverer: discoverer,
			interval:   defaultInterval,
			intervalC:  nil, // note: nil channels are safe to read from (always blocked)
			triggerC:   nil, // note: nil channels are safe to read from (always blocked)
		}
		for _, opt := range opts {
			opt(dc)
		}
		engine.discoverers = append(engine.discoverers, dc)
	}
}

// NewContinuousEngine returns a new ContinuousEngine, initialized with the given options.
func NewContinuousEngine(opts ...ContinuousEngineOption) *ContinuousEngine {
	engine := &ContinuousEngine{discoverers: []*discovererConfig{}}
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
			discoverer.intervalC,
			discoverer.triggerC,
			discoverer.discoverer,
			results,
		)
	}
}
