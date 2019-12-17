<p align="center">
  <a href="https://sentry.io" target="_blank" align="center">
    <img src="https://sentry-brand.storage.googleapis.com/sentry-logo-black.png" width="280">
  </a>
  <br />
</p>

# Official Sentry SDK for Go

[![Build Status](https://travis-ci.com/getsentry/sentry-go.svg?branch=master)](https://travis-ci.com/getsentry/sentry-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/getsentry/sentry-go)](https://goreportcard.com/report/github.com/getsentry/sentry-go)
[![Discord](https://img.shields.io/discord/621778831602221064)](https://discord.gg/Ww9hbqr)

`sentry-go` provides a Sentry client implementation for the Go programming language. This is the next line of the Go SDK for [Sentry](https://sentry.io/), intended to replace the `raven-go` package.

> Looking for the old `raven-go` SDK documentation? See the Legacy client section [here](https://docs.sentry.io/clients/go/).
> If you want to start using sentry-go instead, check out the [migration guide](https://docs.sentry.io/platforms/go/migration/).

## Requirements

We verify this package against N-2 recent versions of Go compiler. As of September 2019, those versions are:

* 1.11
* 1.12
* 1.13

## Installation

`sentry-go` can be installed like any other Go library through `go get`:

```bash
$ go get github.com/getsentry/sentry-go
```

If you are already using [Go Modules](https://blog.golang.org/using-go-modules),
the command above will download the latest tagged release of the SDK.

## Usage

A typical usage of the SDK by example:

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

func main() {
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
```

Step-by-step:

1. To use `sentry-go`, import `github.com/getsentry/sentry-go` and initialize the
package with options.

    The most important option is the DSN. It can be set either in code
    (`sentry.ClientOptions.Dsn`) or through the environment variable `SENTRY_DSN`.

    If the DSN is not set, the SDK is effectively disabled and will not send any
    events to Sentry.

    Other optional environment variables that can we used to configure the SDK
    include `SENTRY_RELEASE` and `SENTRY_ENVIRONMENT`. More on this in the
    [Configuration](https://docs.sentry.io/platforms/go/config/) section of the
    official docs.

2. By default, the Sentry Go SDK uses an asynchronous HTTP transport. That means
that capturing errors, messages and events does not block the current goroutine.
Network communication is done in a separate goroutine.

    As demonstrated in the example above, a call to `sentry.Flush` right before
    the program terminates allows for waiting for events to be delivered to
    Sentry before the process ends. When using the default transport and without
    a call to `Flush` the program process would exit immediately when it reaches
    the end of the `main` function, potentially dropping in-flight events.

    If you would like to change the default behavior and use a synchronous
    transport instead, see the
    [Transports](https://docs.sentry.io/platforms/go/transports) section of the
    official docs. When the `HTTPSyncTransport` is used, `sentry.Flush` is a
    no-op.

3. Report errors with `sentry.CaptureException`.

The SDK also supports integrations that make reporting errors a piece of Gopher
cake when you're building specific types of Go programs.

For more detailed information about how to get the most out of `sentry-go` there is additional documentation available:

- [Configuration](https://docs.sentry.io/platforms/go/config)
- [Error Reporting](https://docs.sentry.io/error-reporting/quickstart?platform=go)
- [Enriching Error Data](https://docs.sentry.io/enriching-error-data/context?platform=go)
- [Transports](https://docs.sentry.io/platforms/go/transports)
- [Integrations](https://docs.sentry.io/platforms/go/integrations)
  - [net/http](https://docs.sentry.io/platforms/go/http)
  - [echo](https://docs.sentry.io/platforms/go/echo)
  - [fasthttp](https://docs.sentry.io/platforms/go/fasthttp)
  - [gin](https://docs.sentry.io/platforms/go/gin)
  - [iris](https://docs.sentry.io/platforms/go/iris)
  - [martini](https://docs.sentry.io/platforms/go/martini)
  - [negroni](https://docs.sentry.io/platforms/go/negroni)

## Resources:

- [Bug Tracker](https://github.com/getsentry/sentry-go/issues)
- [GitHub Project](https://github.com/getsentry/sentry-go)
- [Godocs](https://godoc.org/github.com/getsentry/sentry-go)
- [@getsentry](https://twitter.com/getsentry) on Twitter for updates

## License

Licensed under the BSD license, see `LICENSE`

## Community

Join Sentry's [`#go` channel on Discord](https://discord.gg/Ww9hbqr) to get involved and help us improve the SDK!
