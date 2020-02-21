package sentry

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRequestFromHTTPRequest(t *testing.T) {

	var testPayload = `{"test_data": true}`

	t.Run("reading_body", func(t *testing.T) {
		payload := bytes.NewBufferString(testPayload)
		req, err := http.NewRequest("POST", "/test/", payload)
		assertEqual(t, err, nil)
		assertNotEqual(t, req, nil)
		sentryRequest := NewRequest(req)
		assertEqual(t, sentryRequest.Data, testPayload)

		// Re-reading original *http.Request.Body
		reqBody, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		assertEqual(t, err, nil)
		assertEqual(t, string(reqBody), testPayload)
	})
}

// TODO test GET request, no body

func TestReadRequestBody(t *testing.T) {

	f := f(t)
	err := quick.Check(f, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !f(readRequestBodyInput{[]byte("hello"), 3}) {
		t.Fatal("failed hello test")
	}
}

type readRequestBodyInput struct {
	payload  []byte
	maxBytes int
}

// Generate implements quick.Generator. Returns a random payload of random size
// and random maxBytes within a range based on the payload size.
func (v readRequestBodyInput) Generate(r *rand.Rand, size int) reflect.Value {
	x, ok := quick.Value(reflect.TypeOf(v.payload), r)
	if !ok {
		panic("unreachable")
	}
	v.payload = x.Interface().([]byte)
	v.maxBytes = -10 + r.Intn(len(v.payload)+10) // maxBytes in [-10, 10)
	return reflect.ValueOf(v)
}

func testRequestBody(t *testing.T, payload []byte) {

	// Prepare

	payload := in.payload
	req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))

	// 1. Emulate what the SDK does when it sees an HTTP request.
	r := newRequest(req, maxBytes)

	// 2. Emulate what an SDK user would do in their HTTP handler: read the
	// entire request body (not necessarily into a buffer; could be for instance
	// decoding JSON input, or streaming to disk or another network endpoint).
	finalBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	// 3. Read what is available to the SDK on error, a limited prefix of the
	// original payload.
	limitedBody := r.body.Bytes()

	// Check Invariants

	// 1. Reading the body after a call to readRequestBody should match the
	// original payload.
	if diff := cmp.Diff(payload, finalBody); diff != "" {
		t.Errorf("Request body mismatch on second read (-want +got):\n%s", diff)
	}

	// 2. readRequestBody reads at most maxBytes. If the payload doesn't fit
	// within that limit, it discards the body entirely instead of truncating.
	// That is to avoid cases like sending a truncated partial should either
	// return the
	// ???
	wantLen := max(min(len(payload), maxBytes), 0)
	gotLen := len(limitedBody)
	if diff := cmp.Diff(wantLen, gotLen); diff != "" {
		t.Errorf("Limited request body length mismatch (-want +got):\n%s", diff)
	}

	// 3. ???
	if diff := cmp.Diff(payload[:len(limitedBody)], limitedBody, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Limited request body mismatch (-want +got):\n%s", diff)
	}
}

func f(t *testing.T) func(in readRequestBodyInput) bool {
	return func(in readRequestBodyInput) bool {
		defer func() {
			if v := recover(); false {
				_ = v
			}
		}()
		testReadRequestBody(t, in)
		return !t.Failed()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
