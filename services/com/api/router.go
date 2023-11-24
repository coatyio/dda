// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"sync"
)

const (
	receiveBufferSize = 1 // default buffer size for receive channels
)

// Routable defines a union of communication pattern types that can be routed,
// i.e. dispatched to either a subscriber or a publisher awaiting incoming
// responses.
//
// This type is intended to be used by communication binding implementations.
type Routable interface {
	Event | ActionWithCallback | ActionResult | QueryWithCallback | QueryResult
}

// RouteFilter defines a subscription filter for a specific topic with a
// correlation ID for response topics.
//
// This type is intended to be used by communication binding implementations.
type RouteFilter[T comparable] struct {
	Topic         T // subscription topic
	CorrelationId T // unique correlation ID for response topic only
}

// RouteChannel is a struct representing the receive channel and the unsubscribe
// function of a Routable type.
//
// This type is intended to be used by communication binding implementations.
type RouteChannel[R Routable, T comparable] struct {

	// Channel on which incoming routable data is received.
	ReceiveChan chan R

	// Done channel of the originating context. Function invoked to signal that
	// the subscriber is no longer interested in receiving messages over
	// ReceiveChan. May be invoked multiple times and simultaneously but only
	// the first call will close the receive channel and unsubscribe on the
	// communication binding if necessary.
	CtxDone <-chan struct{}

	correlationId T // Correlation id of response channel, if present

	unsub ComBindingFunc[T] // binding-specific unsubscribe function
}

// ComBindingFunc subscribes, publishes, or unsubscribes a topic of type T
// (captured by the function) on a pub-sub communication binding.
//
// This type is intended to be used by communication binding implementations.
type ComBindingFunc[T comparable] func() error

// Router manages subscription-specific RouteFilters for a specific Routable
// type and dispatches incoming messages on the associated registered receive
// channels. It should be created with NewRouter() to ensure all internal fields
// are correctly populated. Router operations may be invoked concurrently.
//
// This type is intended to be used by communication binding implementations.
type Router[R Routable, T comparable] struct {
	mu     sync.RWMutex                // protects field routes
	routes map[T][]*RouteChannel[R, T] // maps route filters to associated route channels
}

// NewRouter creates a new *Router for the given Routable and RouteFilter type.
func NewRouter[R Routable, T comparable]() *Router[R, T] {
	return &Router[R, T]{
		routes: make(map[T][]*RouteChannel[R, T]),
	}
}

// GetTopics returns all registered RouteFilter Topics in a slice.
func (r *Router[R, T]) GetTopics() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topicSet := make(map[T]struct{}, len(r.routes))
	for topic := range r.routes {
		topicSet[topic] = struct{}{}
	}
	topics := make([]T, len(topicSet))
	i := 0
	for topic := range topicSet {
		topics[i] = topic
		i++
	}
	return topics
}

// Add creates a new RouteChannel of a specific Routable type and registers it
// with the given RouteFilter, if subscription by invoking the subscribe
// function and the publish function is successful. Returns the associated
// *RouteChannel with a receive channel of the given buffer size (if not
// present, defaults to 1), and an unsubscribe function to be invoked by the
// user to deregister the channel and stop receiving data over the channel. If
// subscription and/or publication fails, an error is returned instead.
//
// Note that the publish function should only be used in combination with
// registering response filters. Request filters should always specify a no-op
// publish function.
func (r *Router[R, T]) Add(ctx context.Context, filter RouteFilter[T], subscribe ComBindingFunc[T], publish ComBindingFunc[T], unsubscribe ComBindingFunc[T], bufferSize ...int) (*RouteChannel[R, T], error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	size := receiveBufferSize
	if len(bufferSize) > 0 {
		size = bufferSize[0]
	}
	recvChan := make(chan R, size)
	rc := &RouteChannel[R, T]{ReceiveChan: recvChan, correlationId: filter.CorrelationId}
	rc.unsub = unsubscribe
	rc.CtxDone = ctx.Done()

	if rcs, ok := r.routes[filter.Topic]; ok { // subscription already set up
		if err := publish(); err != nil {
			return nil, err
		}
		r.routes[filter.Topic] = append(rcs, rc)
		context.AfterFunc(ctx, func() {
			r.remove(filter, rc)
		})
		return rc, nil
	} else { // subscription not yet set up
		if err := subscribe(); err != nil {
			return nil, err
		}
		if err := publish(); err != nil {
			_ = unsubscribe() // try unsubscribe as subscription is unused
			return nil, err
		}
		r.routes[filter.Topic] = []*RouteChannel[R, T]{rc}
		context.AfterFunc(ctx, func() {
			r.remove(filter, rc)
		})
		return rc, nil
	}
}

// Dispatch sends the given incoming Routable message on all the registered
// RouteChannels of the associated RouteFilter.
//
// To be used with communication bindings that provide callback functions to
// handle incoming messages.
func (r *Router[R, T]) Dispatch(filter RouteFilter[T], message R) {
	r.dispatch(filter, message)
}

// DispatchChan sends incoming Routable messages received on a channel on all
// the registered RouteChannels of the associated RouteFilter.
//
// To be used with communication bindings that provide channels to receive
// incoming messages.
func (r *Router[R, T]) DispatchChan(filter RouteFilter[T], messages <-chan R) {
	for msg := range messages {
		r.dispatch(filter, msg)
	}
}

func (r *Router[R, T]) dispatch(filter RouteFilter[T], message R) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dispatch := func(rc *RouteChannel[R, T], msg R) {
		// Make sure no more messages are dispatched when associated context has
		// been canceled/timed out but route channel has not yet been removed by
		// context's AfterFunc.
		select {
		case <-rc.CtxDone:
			return
		default:
		}
		select {
		case <-rc.CtxDone:
			return
		case rc.ReceiveChan <- message:
		}
	}
	zerocid := *new(T)
	cid := filter.CorrelationId

	if rcs, ok := r.routes[filter.Topic]; ok {
		if cid == zerocid {
			// Dispatch incoming request to all non-response route channels.
			for _, rc := range rcs {
				if rc.correlationId == zerocid {
					dispatch(rc, message)
				}
			}
		} else {
			// Dispatch incoming response to correlated route channel only.
			for _, rc := range rcs {
				if cid == rc.correlationId {
					dispatch(rc, message)
				}
			}
		}
	}
}

func (r *Router[R, T]) remove(filter RouteFilter[T], routeChannel *RouteChannel[R, T]) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rcs, ok := r.routes[filter.Topic]; ok {
		found := false
		l := len(rcs)
		for i := 0; i < l; i++ {
			if found {
				rcs[i-1] = rcs[i]
			} else if rcs[i] == routeChannel {
				found = true
			}
		}
		if found {
			defer close(routeChannel.ReceiveChan)
			rcs[l-1] = nil
			if l == 1 {
				if err := routeChannel.unsub(); err != nil {
					// As binding subscription is kept, also keep topic without
					// route channel to prevent a subsequent resubscription from
					// failing.
					r.routes[filter.Topic] = rcs[:0]
					return
				}
				delete(r.routes, filter.Topic)
				return
			}
			r.routes[filter.Topic] = rcs[:l-1]
			return
		}
	}
}
