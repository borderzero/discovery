package discovery

import "context"

// Discoverer represents an entity capable of discovering resources.
type Discoverer interface {
	Discover(context.Context) *Result
}
