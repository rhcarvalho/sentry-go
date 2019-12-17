package sentry_test

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

func Example() {
	err := sentry.Init(sentry.ClientOptions{
		// Either set your DSN here or set the environment variable SENTRY_DSN
		// and omit this field altogether.
		Dsn: "",
		// Set Debug to true if you want the SDK to output debug messages. Can
		// be useful when you're getting started.
		//Debug: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sentry.Init: %v\n", err)
		os.Exit(1)
	}
	// Wait until in-flight errors are reported to Sentry before the program
	// exists.
	defer func() {
		const timeout = 10 * time.Second
		ok := sentry.Flush(timeout)
		if !ok {
			fmt.Fprintf(os.Stderr, "sentry.Flush: timed out after %v, some events were not sent\n", timeout)
		}
	}()

	_, err = os.Open("unicorns.txt")
	if err != nil {
		id := "unknown"
		eventID := sentry.CaptureException(err)
		if eventID != nil {
			id = string(*eventID)
		}
		fmt.Fprintf(os.Stderr, "Could not open file: %v\nError reported to Sentry with ID = %v\n", err, id)
	}
}
