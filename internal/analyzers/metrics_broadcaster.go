package analyzers

import (
	"context"
	"log/slog"
)

// MetricsBroadcaster implements the broadcast server pattern from GO_BROADCAST.md
// Allows multiple subscribers to receive metrics events without shared state
type MetricsBroadcaster struct {
	source         <-chan MetricsEvent
	listeners      []chan MetricsEvent
	addListener    chan chan MetricsEvent
	removeListener chan (<-chan MetricsEvent)
}

// NewMetricsBroadcaster creates a new metrics broadcaster
func NewMetricsBroadcaster(source <-chan MetricsEvent) *MetricsBroadcaster {
	return &MetricsBroadcaster{
		source:         source,
		listeners:      make([]chan MetricsEvent, 0),
		addListener:    make(chan chan MetricsEvent),
		removeListener: make(chan (<-chan MetricsEvent)),
	}
}

// Serve runs the broadcast server following the pattern from GO_BROADCAST.md
func (mb *MetricsBroadcaster) Serve(ctx context.Context) {
	defer mb.cleanup()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-mb.source:
			if !ok {
				return // Source channel closed
			}
			mb.broadcast(event)
		case newListener := <-mb.addListener:
			mb.listeners = append(mb.listeners, newListener)
			slog.Debug("Metrics subscriber added", "total_subscribers", len(mb.listeners))
		case listenerToRemove := <-mb.removeListener:
			mb.removeListenerFromSlice(listenerToRemove)
			slog.Debug("Metrics subscriber removed", "total_subscribers", len(mb.listeners))
		}
	}
}

// Subscribe creates a new subscription to metrics events
func (mb *MetricsBroadcaster) Subscribe() <-chan MetricsEvent {
	listener := make(chan MetricsEvent, 100) // Buffered to prevent blocking
	mb.addListener <- listener
	return listener
}

// Unsubscribe cancels a metrics subscription
func (mb *MetricsBroadcaster) Unsubscribe(subscription <-chan MetricsEvent) {
	mb.removeListener <- subscription
}

// broadcast sends events to all subscribers without blocking
func (mb *MetricsBroadcaster) broadcast(event MetricsEvent) {
	// Use non-blocking sends to prevent slow subscribers from blocking others
	for i := len(mb.listeners) - 1; i >= 0; i-- {
		select {
		case mb.listeners[i] <- event:
			// Event sent successfully
		default:
			// Listener is not ready - remove it to prevent memory leaks
			slog.Warn("Removing slow metrics subscriber", "subscriber_index", i)
			mb.removeListenerByIndex(i)
		}
	}
}

// removeListenerFromSlice removes a specific listener from the slice
func (mb *MetricsBroadcaster) removeListenerFromSlice(listenerToRemove <-chan MetricsEvent) {
	for i, listener := range mb.listeners {
		if listener == listenerToRemove {
			mb.removeListenerByIndex(i)
			close(listener) // Close the channel to signal completion
			break
		}
	}
}

// removeListenerByIndex removes a listener by index (efficient removal)
func (mb *MetricsBroadcaster) removeListenerByIndex(index int) {
	// Swap with last element and remove (order doesn't matter for broadcast)
	lastIndex := len(mb.listeners) - 1
	if index != lastIndex {
		mb.listeners[index] = mb.listeners[lastIndex]
	}
	mb.listeners = mb.listeners[:lastIndex]
}

// cleanup closes all listener channels
func (mb *MetricsBroadcaster) cleanup() {
	for _, listener := range mb.listeners {
		close(listener)
	}
	mb.listeners = nil
}
