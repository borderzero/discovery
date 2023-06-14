package discovery

import "context"

// Engine represents an entity capable of managing discovery jobs.
//
// An Engine has three responsibilities:
// - Write zero or more results to the channel
// - Close the channel as soon as they are done with it
// - Exit gracefully upon the context being done
type Engine interface {
	Run(context.Context, chan<- *Result)
}
