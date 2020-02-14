package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

type handler struct{}

func (h *handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	log.Printf("request body type => %T <=", r.Body)
	size, err := io.Copy(ioutil.Discard, r.Body)
	log.Printf("request body size = %d\nerr = %v\nContent-Length = %v", size, err, r.ContentLength)
	if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetExtra("unwantedQuery", "someQueryDataMaybe")
			hub.CaptureMessage("User provided unwanted query string, but we recovered just fine")
		})
	}
	rw.WriteHeader(http.StatusOK)
}

func enhanceSentryEvent(handler http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.Scope().SetTag("someRandomTag", "maybeYouNeedIt")
		}
		handler(rw, r)
	}
}

func main() {
	_ = sentry.Init(sentry.ClientOptions{
		// Dsn: "https://363a337c11a64611be4845ad6e24f3ac@sentry.io/297378",
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if hint.Context != nil {
				if req, ok := hint.Context.Value(sentry.RequestContextKey).(*http.Request); ok {
					// You have access to the original Request
					fmt.Println(req)
				}
			}
			fmt.Println(event)
			return event
		},
		// Debug:            true,
		// AttachStacktrace: true,
	})

	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic: true,
	})

	// http.Handle("/", sentryHandler.Handle(&handler{}))
	http.HandleFunc("/foo", sentryHandler.HandleFunc(
		enhanceSentryEvent(func(rw http.ResponseWriter, r *http.Request) {
			panic("y tho")
		}),
	))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		do(r)
	})

	http.HandleFunc("/s", func(w http.ResponseWriter, r *http.Request) {
		max, _ := strconv.ParseInt(r.URL.Query().Get("max"), 10, 64)
		s := sentry.XreadRequestBody(r, max)
		log.Printf("\n\tmax = %d\n\tlen(s) = %d\n\ts = %q", max, len(s), s)
		do(r)
	})

	fmt.Println("Listening and serving HTTP on :3000")

	if err := http.ListenAndServe("localhost:3000", nil); err != nil {
		panic(err)
	}
}

func do(r *http.Request) {
	log.Printf("request body type => %T <=", r.Body)
	var buf bytes.Buffer
	size, err := io.Copy(&buf, r.Body)
	log.Printf("\n\trequest body size = %d\n\tbody = %q\n\terr = %v\n\tContent-Length = %v", size, buf.String(), err, r.ContentLength)
}
