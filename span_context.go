package sentry

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// A SpanOption is a function that can modify the properties of a span.
type SpanOption func(s *Span)

// The TransactionName option sets the name of the current transaction.
//
// A span tree has a single transaction name, therefore using this option when
// starting a span affects the span tree as a whole, potentially overwriting a
// name set previously.
func TransactionName(name string) SpanOption {
	return func(s *Span) {
		HubFromContext(s.Context()).Scope().SetTransaction(name)
	}
}

// ContinueFromRequest returns a span option that updates the span to continue
// an existing trace. If it cannot detect an existing trace in the request, the
// span will be left unchanged.
func ContinueFromRequest(r *http.Request) SpanOption {
	return func(s *Span) {
		trace := r.Header.Get("sentry-trace")
		if trace == "" {
			return
		}
		s.updateFromSentryTrace([]byte(trace))
	}
}

// sentryTracePattern matches either
//
// 	TRACE_ID - SPAN_ID
// 	[[:xdigit:]]{32}-[[:xdigit:]]{16}
//
// or
//
// 	TRACE_ID - SPAN_ID - SAMPLED
// 	[[:xdigit:]]{32}-[[:xdigit:]]{16}-[01]
var sentryTracePattern = regexp.MustCompile(`^([[:xdigit:]]{32})-([[:xdigit:]]{16})(?:-([01]))?$`)

// updateFromSentryTrace parses a sentry-trace HTTP header (as returned by
// ToSentryTrace) and updates fields of the span. If the header cannot be
// recognized as valid, the span is left unchanged.
func (s *Span) updateFromSentryTrace(header []byte) {
	m := sentryTracePattern.FindSubmatch(header)
	if m == nil {
		// no match
		return
	}
	_, _ = hex.Decode(s.TraceID[:], m[1])
	_, _ = hex.Decode(s.ParentSpanID[:], m[2])
	if len(m[3]) != 0 {
		switch m[3][0] {
		case '0':
			s.Sampled = SampledFalse
		case '1':
			s.Sampled = SampledTrue
		}
	}
}

// ToSentryTrace returns the trace propagation value used with the sentry-trace
// HTTP header.
func (s *Span) ToSentryTrace() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s-%s", s.TraceID.Hex(), s.SpanID.Hex())
	switch s.Sampled {
	case SampledTrue:
		b.WriteString("-1")
	case SampledFalse:
		b.WriteString("-0")
	}
	return b.String()
}

type Sampled int8

// The possible trace sampling decisions are: SampledFalse, SampledUndefined
// (default) and SampledTrue.
const (
	SampledFalse Sampled = -1 + iota
	SampledUndefined
	SampledTrue
)

func (s Sampled) String() string {
	switch s {
	case SampledFalse:
		return "SampledFalse"
	case SampledUndefined:
		return "SampledUndefined"
	case SampledTrue:
		return "SampledTrue"
	default:
		return fmt.Sprintf("SampledInvalid(%d)", s)
	}
}
