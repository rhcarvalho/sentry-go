package sentry

import (
	"sync"
	"time"
)

type ScopeMock struct {
	breadcrumb      *Breadcrumb
	shouldDropEvent bool
}

func (scope *ScopeMock) AddBreadcrumb(breadcrumb *Breadcrumb, limit int) {
	scope.breadcrumb = breadcrumb
}

func (scope *ScopeMock) ApplyToEvent(event *Event, hint *EventHint) *Event {
	if scope.shouldDropEvent {
		return nil
	}
	return event
}

type TransportMock struct {
	mu            sync.Mutex
	events        []*Event
	lastEvent     *Event
	clientOptions *ClientOptions
}

func (t *TransportMock) Configure(options ClientOptions) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clientOptions = &options
}
func (t *TransportMock) SendEvent(event *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !sample(t.clientOptions.SampleRate) {
		return
	}
	t.events = append(t.events, event)
	t.lastEvent = event
}
func (t *TransportMock) Flush(timeout time.Duration) bool {
	return true
}
func (t *TransportMock) Events() []*Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.events
}
