package discovery

import "context"

// Discoverer represents an entity capable of discovering resources.
//
// Discoverers all have three responsibilities:
// - Write zero or more results to the channel
// - Close the channel as soon as they are done with it
// - Exit gracefully upon the context being cancelled/done
type Discoverer interface {
	Discover(context.Context, chan<- *Result)
}
