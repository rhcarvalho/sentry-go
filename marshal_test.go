package sentry

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var (
	generate = flag.Bool("gen", false, "generate missing files in testdata")
	update   = flag.Bool("update", false, "update files in testdata")
)

var (
	goReleaseDate = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	utcMinusTwo   = time.FixedZone("UTC-2", -2*60*60)
)

func TestEventMarshalJSON(t *testing.T) {
	event := NewEvent()
	event.Spans = []*Span{{
		TraceID:        "d6c4f03650bd47699ec65c84352b6208",
		SpanID:         "1cc4b26ab9094ef0",
		ParentSpanID:   "442bd97bbe564317",
		StartTimestamp: time.Unix(8, 0).UTC(),
		EndTimestamp:   time.Unix(10, 0).UTC(),
		Status:         "ok",
	}}
	event.StartTimestamp = time.Unix(7, 0).UTC()
	event.Timestamp = time.Unix(14, 0).UTC()

	got, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	// Non transaction event should not have fields Spans and StartTimestamp
	want := `{"sdk":{},"user":{},"timestamp":"1970-01-01T00:00:14Z"}`

	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("Event mismatch (-want +got):\n%s", diff)
	}
}

func TestStructSnapshots(t *testing.T) {
	testSpan := &Span{
		TraceID:      "d6c4f03650bd47699ec65c84352b6208",
		SpanID:       "1cc4b26ab9094ef0",
		ParentSpanID: "442bd97bbe564317",
		Description:  `SELECT * FROM user WHERE "user"."id" = {id}`,
		Op:           "db.sql",
		Tags: map[string]string{
			"function_name":  "get_users",
			"status_message": "MYSQL OK",
		},
		StartTimestamp: time.Unix(0, 0).UTC(),
		EndTimestamp:   time.Unix(5, 0).UTC(),
		Status:         "ok",
		Data: map[string]interface{}{
			"related_ids":  []uint{12312342, 76572, 4123485},
			"aws_instance": "ca-central-1",
		},
	}

	testCases := []struct {
		testName     string
		sentryStruct interface{}
	}{
		{
			testName:     "span",
			sentryStruct: testSpan,
		},
		{
			testName: "error_event",
			sentryStruct: &Event{
				Message:     "event message",
				Environment: "production",
				EventID:     EventID("0123456789abcdef"),
				Fingerprint: []string{"abcd"},
				Level:       LevelError,
				Platform:    "myplatform",
				Release:     "myrelease",
				Sdk: SdkInfo{
					Name:         "sentry.go",
					Version:      "0.0.1",
					Integrations: []string{"gin", "iris"},
					Packages: []SdkPackage{{
						Name:    "sentry-go",
						Version: "0.0.1",
					}},
				},
				ServerName:  "myhost",
				Timestamp:   time.Unix(5, 0).UTC(),
				Transaction: "mytransaction",
				User:        User{ID: "foo"},
				Breadcrumbs: []*Breadcrumb{{
					Data: map[string]interface{}{
						"data_key": "data_val",
					},
				}},
				Extra: map[string]interface{}{
					"extra_key": "extra_val",
				},
				Contexts: map[string]interface{}{
					"context_key": "context_val",
				},
			},
		},
		{
			testName: "transaction_event",
			sentryStruct: &Event{
				Type:           transactionType,
				Spans:          []*Span{testSpan},
				StartTimestamp: time.Unix(3, 0).UTC(),
				Timestamp:      time.Unix(5, 0).UTC(),
				Contexts: map[string]interface{}{
					"trace": TraceContext{
						TraceID:     "90d57511038845dcb4164a70fc3a7fdb",
						SpanID:      "f7f3fd754a9040eb",
						Op:          "http.GET",
						Description: "description",
						Status:      "ok",
					},
				},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.testName, func(t *testing.T) {
			got, err := json.MarshalIndent(test.sentryStruct, "", "    ")
			if err != nil {
				t.Error(err)
			}

			golden := filepath.Join("testdata", fmt.Sprintf("%s.golden", test.testName))
			if *update {
				err := ioutil.WriteFile(golden, got, 0600)
				if err != nil {
					t.Fatal(err)
				}
			}

			want, err := ioutil.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("struct %s mismatch (-want +got):\n%s", test.testName, diff)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		in  interface{}
		out string
	}{
		// TODO: eliminate empty struct fields from serialization of empty event.
		// Only *Event implements json.Marshaler.
		// {Event{}, `{"sdk":{},"user":{}}`},
		{&Event{}, `{"sdk":{},"user":{}}`},
		// Only *Breadcrumb implements json.Marshaler.
		// {Breadcrumb{}, `{}`},
		{&Breadcrumb{}, `{}`},
	}
	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			want := tt.out
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			got := string(b)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("JSON serialization mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestErrorEventMarshalJSON(t *testing.T) {
	tests := []*Event{
		{
			Message:   "test",
			Timestamp: goReleaseDate,
		},
		{
			Message:   "test",
			Timestamp: goReleaseDate.In(utcMinusTwo),
		},
		{
			Message: "test",
		},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	for i, tt := range tests {
		i, tt := i, tt
		t.Run("", func(t *testing.T) {
			defer buf.Reset()
			err := enc.Encode(tt)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("event", fmt.Sprintf("%03d.json", i))
			if *update {
				WriteGoldenFile(t, path, buf.Bytes())
			}
			got := buf.String()
			want := ReadOrGenerateGoldenFile(t, path, buf.Bytes())
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("JSON mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTransactionEventMarshalJSON(t *testing.T) {
	tests := []*Event{
		{
			Type:           transactionType,
			StartTimestamp: goReleaseDate.Add(-time.Minute),
			Timestamp:      goReleaseDate,
		},
		{
			Type:           transactionType,
			StartTimestamp: goReleaseDate.Add(-time.Minute).In(utcMinusTwo),
			Timestamp:      goReleaseDate.In(utcMinusTwo),
		},
		{
			Type: transactionType,
		},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	for i, tt := range tests {
		i, tt := i, tt
		t.Run("", func(t *testing.T) {
			defer buf.Reset()
			err := enc.Encode(tt)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("transaction", fmt.Sprintf("%03d.json", i))
			if *update {
				WriteGoldenFile(t, path, buf.Bytes())
			}
			got := buf.String()
			want := ReadOrGenerateGoldenFile(t, path, buf.Bytes())
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("MarshalJSON (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBreadcrumbMarshalJSON(t *testing.T) {
	tests := []*Breadcrumb{
		// complete
		{
			Type:     "default",
			Category: "sentryhttp",
			Message:  "breadcrumb message",
			Data: map[string]interface{}{
				"key": "value",
			},
			Level:     LevelInfo,
			Timestamp: goReleaseDate,
		},
		// timestamp not in UTC
		{
			Data: map[string]interface{}{
				"key": "value",
			},
			Timestamp: goReleaseDate.In(utcMinusTwo),
		},
		// missing timestamp
		{
			Message: "breadcrumb message",
		},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	for i, tt := range tests {
		i, tt := i, tt
		t.Run("", func(t *testing.T) {
			defer buf.Reset()
			err := enc.Encode(tt)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("breadcrumb", fmt.Sprintf("%03d.json", i))
			if *update {
				WriteGoldenFile(t, path, buf.Bytes())
			}
			got := buf.String()
			want := ReadOrGenerateGoldenFile(t, path, buf.Bytes())
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("MarshalJSON (-want +got):\n%s", diff)
			}
		})
	}
}

func WriteGoldenFile(t *testing.T, path string, bytes []byte) {
	t.Helper()
	path = filepath.Join("testdata", "marshal", path)
	err := os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(path, bytes, 0666)
	if err != nil {
		t.Fatal(err)
	}
}

func ReadOrGenerateGoldenFile(t *testing.T, path string, bytes []byte) string {
	t.Helper()
	path = filepath.Join("testdata", "marshal", path)
	b, err := ioutil.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if *generate {
			WriteGoldenFile(t, path, bytes)
			return string(bytes)
		}
		t.Fatalf("Missing golden file %q. Run `go test -args -gen` to generate it.", path)
	case err != nil:
		t.Fatal(err)
	}
	return string(b)
}
