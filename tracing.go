package sentry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	mrand "math/rand"
	"time"
)

// A Span is the building block of a Sentry transaction. Spans build up a tree
// structure of timed operations. The span tree makes up a transaction event
// that is sent to Sentry when the root span is finished.
//
// Spans must be started with either StartSpan or Span.StartChild.
type Span struct {
	TraceID      TraceID                `json:"trace_id"`
	SpanID       SpanID                 `json:"span_id"`
	ParentSpanID SpanID                 `json:"parent_span_id"`
	Op           string                 `json:"op,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Status       SpanStatus             `json:"status,omitempty"`
	Tags         map[string]string      `json:"tags,omitempty"`
	StartTime    time.Time              `json:"start_timestamp"`
	EndTime      time.Time              `json:"timestamp"`
	Data         map[string]interface{} `json:"data,omitempty"`

	// TransactionName sets the name of the transaction. Only relevant for the
	// root span of a span tree.
	// transactionName string `json:"-"`
	// TODO: delete this, the transaction name is stored in the scope.

	Sampled Sampled `json:"-"`

	// ctx is the context where the span was started. Always non-nil.
	ctx context.Context

	// parent refers to the immediate local parent span. A remote parent span is
	// only referenced by setting ParentSpanID.
	// TODO: what happens when parent.SpanID and ParentSpanID are different,
	// which takes precedence?
	parent *Span

	// isTransaction is true only for the root span of a local span tree. The
	// root span is the first span started in a context. Note that a local root
	// span may have a remote parent belonging to the same trace, therefore
	// isTransaction depends on ctx and not on parent.
	isTransaction bool

	// recorder stores all spans in a transaction. Guaranteed to be non-nil.
	recorder *spanRecorder
}

// TODO: make Span.Tags and Span.Data opaque types (struct{unexported []slice}).
// An opaque type allows us to add methods and make it more convenient to use
// than maps, because maps require careful nil checks to use properly or rely on
// explicit initialization for every span, even when there might be no
// tags/data. For Span.Data, must gracefully handle values that cannot be
// marshaled into JSON (see transport.go:getRequestBodyFromEvent).

// StartSpan starts a new span to describe an operation. The new span will be a
// child of the last span stored in ctx, if any.
//
// One or more options can be used to modify the span properties. Typically one
// option as a function literal is enough. Combining multiple options can be
// useful to define and reuse specific properties with named functions.
//
// Caller should call the Finish method on the span to mark its end. Finishing a
// root span sends the span and all of its children, recursively, as a
// transaction to Sentry.
func StartSpan(ctx context.Context, operation string, options ...SpanOption) *Span {
	parent, hasParent := ctx.Value(spanContextKey{}).(*Span)
	var span Span
	span = Span{
		// defaults
		Op:        operation,
		StartTime: time.Now(),

		ctx:           context.WithValue(ctx, spanContextKey{}, &span),
		parent:        parent,
		isTransaction: !hasParent,
	}
	if hasParent {
		span.TraceID = parent.TraceID
	} else {
		_, err := rand.Read(span.TraceID[:]) // TODO: custom RNG
		// TODO: is there any perf benefit from doing crypto/rand to generate a
		// seed to use with math/rand later? => math/rand is ~2x faster than
		// crypto/rand
		// https://github.com/open-telemetry/opentelemetry-go/blob/master/sdk/trace/trace.go
		// AFAICT there is no "security" benefit
		// https://github.com/golang/go/issues/11871#issuecomment-126333686
		// https://github.com/golang/go/issues/11871#issuecomment-126357889
		// If we seed math/rand often, the IDs it generate are not nearly as
		// random as UUIDs
		// https://en.wikipedia.org/wiki/Universally_unique_identifier#Collisions
		// only 64 random bits (seed is uint64) instead of 122 from UUIDv4
		// https://www.wolframalpha.com/input/?i=sqrt%282*2%5E64*ln%281%2F%281-0.5%29%29%29
		if err != nil {
			panic(err)
		}
	}
	_, err := rand.Read(span.SpanID[:]) // TODO: custom RNG
	if err != nil {
		panic(err)
	}
	if hasParent {
		span.ParentSpanID = parent.SpanID
	}

	// Apply options to override defaults.
	for _, option := range options {
		option(&span)
	}

	if span.sample() {
		span.Sampled = SampledTrue
	} else {
		span.Sampled = SampledFalse
	}

	if hasParent {
		span.recorder = parent.spanRecorder()
		if span.recorder == nil {
			panic("should never happen") // TODO: should we not panic instead?
		}
	} else {
		span.recorder = &spanRecorder{}
	}
	span.recorder.record(&span)

	// Update scope so that all events include a trace context, allowing Sentry
	// to correlate errors to transactions/spans.
	HubFromContext(ctx).Scope().SetContext("trace", span.traceContext())

	return &span
}

func (s *Span) MarshalJSON() ([]byte, error) {
	// span aliases Span to allow calling json.Marshal without an infinite loop.
	// It preserves all fields while none of the attached methods.
	type span Span
	var parentSpanID string
	if s.ParentSpanID != zeroSpanID {
		parentSpanID = s.ParentSpanID.String()
	}
	return json.Marshal(struct {
		*span
		ParentSpanID string `json:"parent_span_id,omitempty"`
	}{
		span:         (*span)(s),
		ParentSpanID: parentSpanID,
	})
}

func (s *Span) sample() bool {
	if s.Sampled != SampledUndefined {
		// Sampling Decision #1 (see
		// https://develop.sentry.dev/sdk/unified-api/tracing/#sampling)
		// Set by user via options.
		return s.Sampled == SampledTrue
	}
	hub := HubFromContext(s.ctx)
	var clientOptions ClientOptions
	client := hub.Client()
	if client != nil {
		clientOptions = hub.Client().Options() // TODO: check nil client
	}
	sampler := clientOptions.TracesSampler
	samplingContext := SamplingContext{Span: s, Parent: s.parent}
	if sampler != nil {
		return sampler.Sample(samplingContext) // Sampling Decision #2
	}
	if s.parent != nil {
		return s.parent.Sampled == SampledTrue // Sampling Decision #3
	}
	sampler = &fixedRateSampler{ // TODO: pre-compute the TracesSampler once and avoid extra computations in StartSpan.
		Rand: mrand.New(mrand.NewSource(1)), // TODO: use proper RNG
		Rate: clientOptions.TracesSampleRate,
	}
	return sampler.Sample(samplingContext) // Sampling Decision #4
}

// Context returns the context containing the span.
func (s *Span) Context() context.Context { return s.ctx }

// Finish sets the span's end time, unless already set. If the span is the root
// of a span tree, Finish sends the span tree to Sentry as a transaction.
func (s *Span) Finish() {
	// FIXME TODO: Finish should not block for a long time; do slow work in a
	// new goroutine
	// FIXME TODO: must limit the number of spans / out-going request size

	if s.EndTime.IsZero() {
		s.EndTime = monotonicTimeSince(s.StartTime)
	}
	if s.Sampled != SampledTrue {
		return
	}
	event := s.toEvent()
	if event == nil {
		return
	}
	hub := HubFromContext(s.ctx)
	// TODO: FIXME accessing the Scope.transaction directly is racy -- bypasses
	// the internal mutex.
	if hub.Scope().transaction == "" {
		Logger.Printf("Missing transaction name for span with op = %q", s.Op)
	}
	hub.CaptureEvent(event)
}

func (s *Span) toEvent() *Event {
	if !s.isTransaction {
		return nil // only transactions can be transformed into events
	}
	hub := HubFromContext(s.ctx)
	// TODO: FIXME accessing the Scope.transaction directly is racy -- bypasses
	// the internal mutex.
	transactionName := hub.Scope().transaction
	return &Event{
		Type:        transactionType,
		Transaction: transactionName,
		Contexts: map[string]interface{}{
			"trace": s.traceContext(),
		},
		Tags:      s.Tags,
		Timestamp: s.EndTime,
		StartTime: s.StartTime,
		Spans:     s.recorder.children(),
	}
}

func (s *Span) traceContext() TraceContext {
	return TraceContext{
		TraceID:      s.TraceID,
		SpanID:       s.SpanID,
		ParentSpanID: s.ParentSpanID,
		Op:           s.Op,
		Description:  s.Description,
		Status:       s.Status,
	}
}

// StartChild starts a new child span.
//
// The call span.StartChild(operation, options...) is a shortcut for
// StartSpan(span.Context(), operation, options...).
func (s *Span) StartChild(operation string, options ...SpanOption) *Span {
	return StartSpan(s.Context(), operation, options...)
}

// spanRecorder stores the span tree. Guaranteed to be non-nil.
func (s *Span) spanRecorder() *spanRecorder { return s.recorder }

// TODO: add these shortcuts or keep transaction name in scope and find
// different way to facilitate get/set transaction name.
// func (s *Span) TransactionName() string
// func (s *Span) SetTransactionName(name string)

// TraceID identifies a trace.
type TraceID [16]byte

func (id TraceID) Hex() []byte {
	b := make([]byte, hex.EncodedLen(len(id)))
	hex.Encode(b, id[:])
	return b
}

func (id TraceID) String() string {
	return string(id.Hex())
}

func (id TraceID) MarshalText() ([]byte, error) {
	return id.Hex(), nil
}

// SpanID identifies a span.
type SpanID [8]byte

func (id SpanID) Hex() []byte {
	b := make([]byte, hex.EncodedLen(len(id)))
	hex.Encode(b, id[:])
	return b
}

func (id SpanID) String() string {
	return string(id.Hex())
}

func (id SpanID) MarshalText() ([]byte, error) {
	return id.Hex(), nil
}

// Zero values of TraceID and SpanID used for comparisons.
var (
	zeroTraceID TraceID
	zeroSpanID  SpanID
)

// SpanStatus is the status of a span.
type SpanStatus uint8

// Implementation note:
//
// In Relay (ingestion), the SpanStatus type is an enum used as
// Annotated<SpanStatus> when embedded in structs, making it effectively
// Option<SpanStatus>. It means the status is either null or one of the known
// string values.
//
// In Snuba (search), the SpanStatus is stored as an uint8 and defaulted to 2
// ("unknown") when not set. It means that Discover searches for
// `transaction.status:unknown` return both transactions/spans with status
// `null` or `"unknown"`. Searches for `transaction.status:""` return nothing.
//
// With that in mind, the Go SDK default is SpanStatusUndefined, which is
// null/omitted when serializing to JSON, but integrations may update the status
// automatically based on contextual information.

const (
	SpanStatusUndefined SpanStatus = iota
	SpanStatusOK
	SpanStatusCanceled
	SpanStatusUnknown
	SpanStatusInvalidArgument
	SpanStatusDeadlineExceeded
	SpanStatusNotFound
	SpanStatusAlreadyExists
	SpanStatusPermissionDenied
	SpanStatusResourceExhausted
	SpanStatusFailedPrecondition
	SpanStatusAborted
	SpanStatusOutOfRange
	SpanStatusUnimplemented
	SpanStatusInternalError
	SpanStatusUnavailable
	SpanStatusDataLoss
	SpanStatusUnauthenticated
	maxSpanStatus
)

func (ss SpanStatus) String() string {
	if ss >= maxSpanStatus {
		return ""
	}
	m := [maxSpanStatus]string{
		"",
		"ok",
		"cancelled", // [sic]
		"unknown",
		"invalid_argument",
		"deadline_exceeded",
		"not_found",
		"already_exists",
		"permission_denied",
		"resource_exhausted",
		"failed_precondition",
		"aborted",
		"out_of_range",
		"unimplemented",
		"internal_error",
		"unavailable",
		"data_loss",
		"unauthenticated",
	}
	return m[ss]
}

func (ss SpanStatus) MarshalJSON() ([]byte, error) {
	s := ss.String()
	if s == "" {
		return []byte("null"), nil
	}
	return json.Marshal(s)
}

// A TraceContext carries information about an ongoing trace and is meant to be
// stored in Event.Contexts (as *TraceContext).
type TraceContext struct {
	TraceID      TraceID    `json:"trace_id"`
	SpanID       SpanID     `json:"span_id"`
	ParentSpanID SpanID     `json:"parent_span_id"`
	Op           string     `json:"op,omitempty"`
	Description  string     `json:"description,omitempty"`
	Status       SpanStatus `json:"status,omitempty"`
}

func (tc *TraceContext) MarshalJSON() ([]byte, error) {
	// traceContext aliases TraceContext to allow calling json.Marshal without
	// an infinite loop. It preserves all fields while none of the attached
	// methods.
	type traceContext TraceContext
	var parentSpanID string
	if tc.ParentSpanID != zeroSpanID {
		parentSpanID = tc.ParentSpanID.String()
	}
	return json.Marshal(struct {
		*traceContext
		ParentSpanID string `json:"parent_span_id,omitempty"`
	}{
		traceContext: (*traceContext)(tc),
		ParentSpanID: parentSpanID,
	})
}

// spanContextKey is used to store span values in contexts.
type spanContextKey struct{}

// spanFromContext returns the last span stored in the context or ........
//
// TODO: ensure this is really needed as public API ---
// 	SpanFromContext(ctx).StartChild(...) === StartSpan(ctx, ...)
// Do we need this for anything else?
// If we remove this we can also remove noopSpan.
// Without this, users cannot retrieve a span from a context since
// spanContextKey is not exported.
// This can be added retroactively, and in the meantime think better what's
// best: return nil or noopSpan (or both: two exported functions).
func spanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return span
	}
	return &Span{ctx: ctx}
}
