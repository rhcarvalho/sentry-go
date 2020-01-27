package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s URL", os.Args[0])
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:   "", // set DSN here or set SENTRY_DSN environment variable
		Debug: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	// Maybe like dotnet Close should take no arguments and use
	// ClientOptions.ShutdownTimeout, default 2s.
	// https://github.com/getsentry/sentry-dotnet/blob/f6007f621d4935ca2570c0163ede944d7c86483f/src/Sentry/SentryOptions.cs#L240
	defer sentry.Close(30 * time.Second) // set the timeout to a value appropriate for your program

	resp, err := http.Get(os.Args[1])
	if err != nil {
		sentry.CaptureException(err)
		log.Printf("reported to Sentry: %s", err)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		for _, v1 := range v {
			fmt.Printf("%s=%s\n", k, v1)
		}
	}
}
