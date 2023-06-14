package engines

import (
	"context"
	"sync"

	"github.com/borderzero/discovery"
)

// OneOffEngine represents an engine to run one-off discovery jobs.
type OneOffEngine struct {
	discoverers []discovery.Discoverer
}

// ensure OneOffEngine implements discovery.Engine at compile-time.
var _ discovery.Engine = (*OneOffEngine)(nil)

// OneOffEngineOption is an input option for the OneOffEngine constructor.
type OneOffEngineOption func(*OneOffEngine)

// OneOffEngineOptionWithDiscoverers is a configuration option to include
// additional Discoverer(s) in a OneOffEngine's discovery jobs.
func OneOffEngineOptionWithDiscoverers(discoverers ...discovery.Discoverer) OneOffEngineOption {
	return func(engine *OneOffEngine) {
		engine.discoverers = append(engine.discoverers, discoverers...)
	}
}

// NewOneOffEngine returns a new OneOffEngine, initialized with the given options.
func NewOneOffEngine(opts ...OneOffEngineOption) *OneOffEngine {
	engine := &OneOffEngine{discoverers: []discovery.Discoverer{}}
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

// Run runs the OneOffEngine and closes the channels after
// a single run of all the underlying discoverers is completed.
func (e *OneOffEngine) Run(
	ctx context.Context,
	results chan<- *discovery.Result,
) {
	defer close(results)

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, discoverer := range e.discoverers {
		wg.Add(1)

		go runOnce(
			ctx,
			&wg,
			discoverer,
			results,
		)
	}
}
