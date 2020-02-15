package sentry

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"testing/quick"
)

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

	cfg := &quick.Config{
		MaxCount: 100000,
	}

	f := f(t)
	err := quick.Check(f, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !f([]byte("hello"), 3) {
		t.Fatal("failed hello test")
	}

	// tests := []struct {
	// 	payload  []byte
	// 	maxBytes int64
	// }{
	// 	{
	// 		payload:  nil,
	// 		maxBytes: 0,
	// 	},
	// }

	// t.Run("new", func(t *testing.T) {
	// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		XreadRequestBody(r, 5)
	// 	}))
	// 	client := server.Client()
	// 	client.Get()
	// })
}

func f(t *testing.T) func(payload []byte, maxBytes int64) bool {
	return func(payload []byte, maxBytes int64) bool {
		if maxBytes < 0 {
			maxBytes = -maxBytes
		}
		t.Logf("maxBytes = %d", maxBytes)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))

		limitedBody := XreadRequestBody(req, maxBytes)
		if maxBytes >= 0 && int64(len(limitedBody)) > maxBytes {
			t.Logf("len(limitedBody) = %d, maxBytes = %d", len(limitedBody), maxBytes)
			return false
		}
		var buf bytes.Buffer
		n, err := io.Copy(&buf, req.Body)
		if err != nil {
			panic("Copy: " + err.Error())
		}
		if n != int64(len(payload)) {
			t.Logf("n = %d, len(payload) = %d", n, len(payload))
			return false
		}
		if !reflect.DeepEqual(buf.Bytes(), payload) {
			t.Logf("body = %q, payload = %q", buf.Bytes(), payload)
			return false
		}

		if n != req.ContentLength {
			t.Logf("n = %d, ContentLength = %d", n, req.ContentLength)
			return false
		}
		return true
	}
}
