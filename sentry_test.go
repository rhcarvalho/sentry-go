package sentry

import (
	"fmt"
	"testing"
	"time"
)

// type testTransport struct {
// 	*HTTPTransport

// 	// CloseHandler func(timeout time.Duration, close func(timeout time.Duration))
// }

// func (t *testTransport) close(timeout time.Duration) {
// 	t.CloseHandler(timeout, t.HTTPTransport.close)
// }

func TestClose(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()

	// closed := make(chan bool)

	Init(ClientOptions{
		Dsn:        fmt.Sprintf("https://test@%s/1", server.Listener.Addr()),
		HTTPClient: server.Client(),
		// Transport: &testTransport{
		// 	HTTPTransport: NewHTTPTransport(),
		// 	// CloseHandler: func(timeout time.Duration, close func(timeout time.Duration)) {
		// 	// 	close(timeout)
		// 	// 	closed <- true
		// 	// },
		// },
		// Debug:      true,
	})

	CaptureMessage("msg 1")
	server.EventCountMustBe(t, 0)
	server.Unblock()

	Close(100 * time.Millisecond)

	server.EventCountMustBe(t, 1)

	CaptureMessage("msg 2")
	server.Unblock()

	Close(100 * time.Millisecond)

	// Count must not have increased as no new message can be sent after Close.
	server.EventCountMustBe(t, 1)
}
