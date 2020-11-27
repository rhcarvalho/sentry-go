package sentryhttp

import (
	"context"
	"net/http"
)

// See also:
// https://github.com/open-telemetry/opentelemetry-go-contrib/tree/master/instrumentation/net/http/otelhttp
// https://github.com/open-telemetry/opentelemetry-go-contrib/blob/master/instrumentation/net/http/otelhttp/example/client/client.go
//
// OpenTelemetry allows user code to use the standard net/http.Client with a
// custom transport, as long as you use net/http.NewRequestWithContext and
// Client.Do, and pass in the correct context.
//
// The idea here was to expose shortcuts sentryhttp.Get(ctx, ...),
// sentryhttp.Post(ctx, ...) to replace http.Get, http.Post, etc.
//
// Either way, users still need to change their code to make instrumentation
// work. It won't work without user cooperation. We are not able to make
// arbitrary user libraries propagate trace information.
//
// Note that users could mutate http.DefaultClient (not pretty), but that
// doesn't solve the problem, as we still need proper context propagation (and
// absolutely prohibit uses of non-context-aware functions like http.Get, etc).

func NewTransport(rt http.RoundTripper) http.RoundTripper {
	// TODO(tracing): instrument rt to create spans for outgoing requests.
	return rt
}

// defaultClient is the http.Client used by Get, Head, and Post.
//
// To customize the client, create a new http.Client and use NewTransport to
// wrap the client's transport.
var defaultClient = &http.Client{Transport: NewTransport(http.DefaultTransport)}

// Get issues a GET to the specified URL. It is a shortcut for http.Get with a
// context.
//
// See the Go standard library documentation for net/http for details.
//
// When err is nil, resp always contains a non-nil resp.Body.
// Caller should close resp.Body when done reading from it.
//
// To make a custom request, create a client with a transport wrapped by
// NewTransport and use http.NewRequestWithContext and http.Client.Do.
func Get(ctx context.Context, url string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return defaultClient.Do(req)
}
