package engines

import (
	"context"
	"sync"
	"time"

	"github.com/borderzero/discovery"
)

// runContinuously runs a discoverer continuously and signals a wait
// group when done (which will only be when the context is done).
// ** Note that it does not close the results channel **
func runContinuously(
	ctx context.Context,
	wg *sync.WaitGroup,
	interval time.Duration,
	discoverer discovery.Discoverer,
	results chan<- *discovery.Result,
) {
	defer wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var innerWg sync.WaitGroup
	defer innerWg.Wait()

	innerWg.Add(1)
	go runOnce(ctx, &innerWg, discoverer, results)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			innerWg.Add(1)
			go runOnce(ctx, &innerWg, discoverer, results)
		}
	}
}

// runOnce runs a discoverer just once and signals a wait group
// when done (which will only be when the context is done).
// ** Note that it does not close the results channel **
func runOnce(
	ctx context.Context,
	wg *sync.WaitGroup,
	discoverer discovery.Discoverer,
	results chan<- *discovery.Result,
) {
	defer wg.Done()

	resultsInner := make(chan *discovery.Result, cap(results))

	go discoverer.Discover(ctx, resultsInner)

	for result := range resultsInner {
		results <- result
	}
}
