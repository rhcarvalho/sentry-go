package sentry_test

import (
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestClientConcurrency(t *testing.T) {
	client, err := sentry.NewClient(sentry.ClientOptions{
		Transport: sentry.NewHTTPTransport(),
	})
	if err != nil {
		t.Fatal(err)
	}
	hub1 := sentry.NewHub(client, sentry.NewScope())
	hub2 := hub1.Clone()
	if !(hub1.Client() == client && hub2.Client() == client) {
		t.Fatal("clients not wired up correctly")
	}
	dropAll := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event { return nil }
	go func() {
		hub1.Client().AddEventProcessor(dropAll)                // DATA RACE: mutation of Client.eventProcessors
		hub1.Client().Transport = sentry.NewHTTPSyncTransport() // DATA RACE: mutation of Client.Transport
	}()
	hub2.CaptureMessage("hello 2")
}
