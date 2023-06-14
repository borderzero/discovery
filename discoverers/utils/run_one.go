package utils

import (
	"context"
	"sync"
	"time"

	"github.com/borderzero/discovery"
)

// RunContinuously runs a discoverer continuously and signals a
// wait group when done (which will only be when the context is done)
func RunContinuously(
	ctx context.Context,
	wg *sync.WaitGroup,
	interval time.Duration,
	discoverer discovery.Discoverer,
	results chan<- *discovery.Result,
) {
	defer wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go runOnce(ctx, discoverer, results)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go runOnce(ctx, discoverer, results)
		}
	}
}

// RunOneOff runs a discoverer just once and signals a wait
// group when done (which will be when the discoverer is done)
func RunOneOff(
	ctx context.Context,
	wg *sync.WaitGroup,
	discoverer discovery.Discoverer,
	results chan<- *discovery.Result,
) {
	defer wg.Done()
	runOnce(ctx, discoverer, results)
}

// runOnce runs the discoverer just once with a locally defined results channel. This
// is useful because the channel given to runOnce() will not be closed by the discoverer.
func runOnce(
	ctx context.Context,
	discoverer discovery.Discoverer,
	results chan<- *discovery.Result,
) {
	resultsInner := make(chan *discovery.Result, len(results))

	go discoverer.Discover(ctx, resultsInner)

	for result := range resultsInner {
		results <- result
	}
}
