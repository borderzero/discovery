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
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	defer wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go runOnce(ctx, discoverer, resources, errors)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go runOnce(ctx, discoverer, resources, errors)
		}
	}
}

// RunOneOff runs a discoverer just once and signals a wait
// group when done (which will be when the discoverer is done)
func RunOneOff(
	ctx context.Context,
	wg *sync.WaitGroup,
	discoverer discovery.Discoverer,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	defer wg.Done()
	runOnce(ctx, discoverer, resources, errors)
}

// runOnce runs the discoverer just once with locally defined channels.
// This is useful because the given channels will not be closed by the discoverer.
func runOnce(
	ctx context.Context,
	discoverer discovery.Discoverer,
	resources chan<- []discovery.Resource,
	errors chan<- error,
) {
	resourcesInner := make(chan []discovery.Resource, len(resources))
	errorsInner := make(chan error, len(errors))

	go func() {
		for resourcesInnerBatch := range resourcesInner {
			resources <- resourcesInnerBatch
		}
	}()

	go func() {
		for errorInner := range errorsInner {
			errors <- errorInner
		}
	}()

	discoverer.Discover(ctx, resourcesInner, errorsInner)
}
