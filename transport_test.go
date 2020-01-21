package sentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type unserializableType struct {
	UnsupportedField func()
}

const basicEvent = "{\"message\":\"mkey\",\"sdk\":{},\"user\":{},\"request\":{}}"
const enhancedEvent = "{\"extra\":{\"info\":\"Original event couldn't be marshalled. Succeeded by stripping " +
	"the data that uses interface{} type. Please verify that the data you attach to the scope is serializable.\"}," +
	"\"message\":\"mkey\",\"sdk\":{},\"user\":{},\"request\":{}}"

func TestGetRequestBodyFromEventValid(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Message: "mkey",
	})

	got := string(body)
	want := basicEvent

	if got != want {
		t.Errorf("expected different shape of body. \ngot: %s\nwant: %s", got, want)
	}
}

func TestGetRequestBodyFromEventInvalidBreadcrumbsField(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Message: "mkey",
		Breadcrumbs: []*Breadcrumb{{
			Data: map[string]interface{}{
				"wat": unserializableType{},
			},
		}},
	})

	got := string(body)
	want := enhancedEvent

	if got != want {
		t.Errorf("expected different shape of body. \ngot: %s\nwant: %s", got, want)
	}
}

func TestGetRequestBodyFromEventInvalidExtraField(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Message: "mkey",
		Extra: map[string]interface{}{
			"wat": unserializableType{},
		},
	})

	got := string(body)
	want := enhancedEvent

	if got != want {
		t.Errorf("expected different shape of body. \ngot: %s\nwant: %s", got, want)
	}
}

func TestGetRequestBodyFromEventInvalidContextField(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Message: "mkey",
		Contexts: map[string]interface{}{
			"wat": unserializableType{},
		},
	})

	got := string(body)
	want := enhancedEvent

	if got != want {
		t.Errorf("expected different shape of body. \ngot: %s\nwant: %s", got, want)
	}
}

func TestGetRequestBodyFromEventMultipleInvalidFields(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Message: "mkey",
		Breadcrumbs: []*Breadcrumb{{
			Data: map[string]interface{}{
				"wat": unserializableType{},
			},
		}},
		Extra: map[string]interface{}{
			"wat": unserializableType{},
		},
		Contexts: map[string]interface{}{
			"wat": unserializableType{},
		},
	})

	got := string(body)
	want := enhancedEvent

	if got != want {
		t.Errorf("expected different shape of body. \ngot: %s\nwant: %s", got, want)
	}
}

func TestGetRequestBodyFromEventCompletelyInvalid(t *testing.T) {
	body := getRequestBodyFromEvent(&Event{
		Exception: []Exception{{
			Stacktrace: &Stacktrace{
				Frames: []Frame{{
					Vars: map[string]interface{}{
						"wat": unserializableType{},
					},
				}},
			},
		}},
	})

	if body != nil {
		t.Error("expected body to be nil")
	}
}

func TestRetryAfterNoHeader(t *testing.T) {
	r := http.Response{}
	assertEqual(t, retryAfter(time.Now(), &r), time.Second*60)
}

func TestRetryAfterIncorrectHeader(t *testing.T) {
	r := http.Response{
		Header: map[string][]string{
			"Retry-After": {"x"},
		},
	}
	assertEqual(t, retryAfter(time.Now(), &r), time.Second*60)
}

func TestRetryAfterDelayHeader(t *testing.T) {
	r := http.Response{
		Header: map[string][]string{
			"Retry-After": {"1337"},
		},
	}
	assertEqual(t, retryAfter(time.Now(), &r), time.Second*1337)
}

func TestRetryAfterDateHeader(t *testing.T) {
	now, _ := time.Parse(time.RFC1123, "Wed, 21 Oct 2015 07:28:00 GMT")
	r := http.Response{
		Header: map[string][]string{
			"Retry-After": {"Wed, 21 Oct 2015 07:28:13 GMT"},
		},
	}
	assertEqual(t, retryAfter(now, &r), time.Second*13)
}

type testWriter testing.T

func (t *testWriter) Write(p []byte) (int, error) {
	t.Logf("%s", p)
	return len(p), nil
}

func TestHTTPTransportFlush(t *testing.T) {
	var counter uint64
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		var e struct {
			EventID string `json:"event_id"`
		}
		err := dec.Decode(&e)
		if err != nil {
			panic(err)
		}
		t.Logf("{%.4s} [SERVER] received event: #%d", e.EventID, atomic.AddUint64(&counter, 1))
	}))
	defer ts.Close()

	Logger.SetOutput((*testWriter)(t))

	tr := NewHTTPTransport()
	tr.Configure(ClientOptions{
		Dsn:        fmt.Sprintf("https://user@%s/42", ts.Listener.Addr()),
		HTTPClient: ts.Client(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2; j++ {
				e := NewEvent()
				e.EventID = EventID(uuid())
				t.Logf("{%.4s} tr.SendEvent #%d from goroutine #%d", e.EventID, j, i)
				tr.SendEvent(e)
				ok := tr.Flush(200 * time.Millisecond)
				if !ok {
					t.Errorf("{%.4s} Flush() timed out", e.EventID)
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkHTTPTransport(b *testing.B) {
	var counter uint64
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		atomic.AddUint64(&counter, 1)
	}))
	defer ts.Close()

	tr := NewHTTPTransport()
	tr.Configure(ClientOptions{
		Dsn:        fmt.Sprintf("https://user@%s/42", ts.Listener.Addr()),
		HTTPClient: ts.Client(),
	})

	e := NewEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 && i%tr.BufferSize == 0 {
			tr.Flush(3000 * time.Millisecond)
		}
		tr.SendEvent(e)
	}
	ok := tr.Flush(2000 * time.Millisecond)
	if !ok {
		b.Error("Flush() timed out")
	}
	if counter != uint64(b.N) {
		b.Errorf("counter = %d, want %d", counter, b.N)
	}
}
func BenchmarkHTTPTransportNoFlush(b *testing.B) {
	var counter uint64
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		atomic.AddUint64(&counter, 1)
	}))
	defer ts.Close()

	tr := NewHTTPTransport()
	tr.Configure(ClientOptions{
		Dsn:        fmt.Sprintf("https://user@%s/42", ts.Listener.Addr()),
		HTTPClient: ts.Client(),
	})

	e := NewEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.SendEvent(e)
	}
	b.StopTimer()
	tr.Flush(time.Second)
	b.Logf("counter = %d, b.N = %d", counter, b.N)
}
func BenchmarkHTTPSyncTransport(b *testing.B) {
	var counter uint64
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		atomic.AddUint64(&counter, 1)
	}))
	defer ts.Close()

	tr := NewHTTPSyncTransport()
	tr.Configure(ClientOptions{
		Dsn:        fmt.Sprintf("https://user@%s/42", ts.Listener.Addr()),
		HTTPClient: ts.Client(),
	})

	e := NewEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.SendEvent(e)
	}
	ok := tr.Flush(200 * time.Millisecond)
	if !ok {
		b.Error("Flush() timed out")
	}
	if counter != uint64(b.N) {
		b.Errorf("counter = %d, want %d", counter, b.N)
	}
}
