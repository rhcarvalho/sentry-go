package sentry

import (
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type readRequestBodyInput struct {
	payload  []byte
	maxBytes int64
}

// Generate implements quick.Generator.
func (v readRequestBodyInput) Generate(r *rand.Rand, size int) reflect.Value {
	x, ok := quick.Value(reflect.TypeOf([]byte(nil)), r)
	if !ok {
		panic("unreachable")
	}
	v.payload = x.Interface().([]byte)
	v.maxBytes = -1 + int64(rand.NewZipf(r, 1.01, 1+float64(len(v.payload)), math.MaxInt64).Uint64())
	return reflect.ValueOf(v)
}

func TestRequestFromHTTPRequest(t *testing.T) {

	var testPayload = `{"test_data": true}`

	t.Run("reading_body", func(t *testing.T) {
		payload := bytes.NewBufferString(testPayload)
		req, err := http.NewRequest("POST", "/test/", payload)
		assertEqual(t, err, nil)
		assertNotEqual(t, req, nil)
		sentryRequest := Request{}
		sentryRequest = sentryRequest.FromHTTPRequest(req)
		assertEqual(t, sentryRequest.Data, testPayload)

		// Re-reading original *http.Request.Body
		reqBody, err := ioutil.ReadAll(req.Body)
		assertEqual(t, err, nil)
		assertEqual(t, string(reqBody), testPayload)
	})
}

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

func testReadRequestBody(t *testing.T, in readRequestBodyInput) {
	// Prepare
	payload, maxBytes := in.payload, in.maxBytes

	originalBody := payload

	req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))

	limitedBody := XreadRequestBody(req, maxBytes)

	var buf bytes.Buffer
	_, err := io.Copy(&buf, req.Body)
	if err != nil {
		panic(err)
	}

	finalBody := buf.Bytes()

	// Check Invariants

	if diff := cmp.Diff(originalBody, finalBody); diff != "" {
		t.Errorf("Request body mismatch on second read (-want +got):\n%s", diff)
	}
	wantLen := max(min(int64(len(originalBody)), maxBytes), 0)
	gotLen := int64(len(limitedBody))
	if diff := cmp.Diff(wantLen, gotLen); diff != "" {
		t.Errorf("Limited request body length mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(originalBody[:len(limitedBody)], limitedBody, cmpopts.EquateEmpty()); diff != "" {
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

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
