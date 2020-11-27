package sentry

import (
	"sync"
)

// maxSpans limits the number of recorded spans per transaction. The limit is
// meant to bound memory usage and prevent too large transaction events that
// would be rejected by Sentry.
const maxSpans = 100

// A spanRecorder stores a span tree that makes up a transaction. Safe for
// concurrent use. It is okay to add child spans from multiple goroutines.
type spanRecorder struct {
	mu    sync.Mutex
	spans []*Span
}

// record stores a span. The first stored span is assumed to be the root of a
// span tree.
func (r *spanRecorder) record(s *Span) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.spans) < maxSpans {
		r.spans = append(r.spans, s)
	}
	// TODO(tracing): notify when maxSpans is reached
}

// children returns a list of all recorded spans, except the root. Returns nil
// if there are no children.
func (r *spanRecorder) children() []*Span {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.spans) < 2 {
		return nil
	}
	return r.spans[1:]
}
