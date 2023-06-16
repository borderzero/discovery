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
	intervalC <-chan time.Duration,
	triggerC <-chan struct{},
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
		// handle context being done
		case <-ctx.Done():
			return
		// handle receiving an interval update
		case newInterval, ok := <-intervalC:
			if ok {
				interval = newInterval
				ticker.Reset(interval)
			}
		// handle receiving a manual run trigger
		case _, ok := <-triggerC:
			if ok {
				ticker.Reset(interval)
				go runOnce(ctx, &innerWg, discoverer, results)
			}
		// handle tick from ticker (run now)
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
	results <- discoverer.Discover(ctx)
}
