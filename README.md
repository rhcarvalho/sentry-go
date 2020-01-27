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

The only requirement is a Go compiler.

We verify this package against the 3 most recent releases of Go. Those are the
supported versions. The exact versions are defined in
[`.travis.yml`](.travis.yml).

In addition, we run tests against the current master branch of the Go toolchain,
though support for this configuration is best-effort.

## Installation

`sentry-go` can be installed like any other Go library through `go get`:

```bash
$ go get github.com/getsentry/sentry-go
```

Or, if you are already using [Go
Modules](https://github.com/golang/go/wiki/Modules), you may specify a version
number as well:

```bash
$ go get github.com/getsentry/sentry-go@latest
```

Check out the [list of released versions](https://pkg.go.dev/github.com/getsentry/sentry-go?tab=versions). 



## Configuration

To use `sentry-go`, you’ll need to import the `sentry-go` package and initialize
it with your DSN and other [options](https://godoc.org/github.com/getsentry/sentry-go#ClientOptions).

If not specified in the SDK initialization, the
[DSN](https://docs.sentry.io/error-reporting/configuration/?platform=go#dsn),
[Release](https://docs.sentry.io/workflow/releases/?platform=go) and
[Environment](https://docs.sentry.io/enriching-error-data/environments/?platform=go)
are read from the environment variables `SENTRY_DSN`, `SENTRY_RELEASE` and
`SENTRY_ENVIRONMENT`, respectively.

More on this in the [Configuration](https://docs.sentry.io/platforms/go/config/)
section of the official Sentry documentation.

## Usage

The SDK must be initialized with a call to `sentry.Init` and must be closed at
the end of the program with `sentry.Close`.

It is important to call `sentry.Close` to release resources and wait until
buffered events are sent to Sentry.

Typically, `sentry.Init` is called in the beginning of `func main` and
`sentry.Close` is [deferred](https://golang.org/ref/spec#Defer_statements) right
after.

> Note that if the program terminates with a call to
> [`os.Exit`](https://golang.org/pkg/os/#Exit), either directly or indirectly
> via another function like `log.Fatal`, deferred functions are not run.
>
> In that case, and if it is important for you to report outstanding events
> before terminating the program, arrange for `sentry.Close` to be called before
> the program terminates.


Example:

```go
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
	defer sentry.Close(2 * time.Second) // set the timeout to a value appropriate for your program

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
```

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

Licensed under [The 2-Clause BSD License](https://opensource.org/licenses/BSD-2-Clause), see [`LICENSE`](LICENSE).

## Community

Join Sentry's [`#go` channel on Discord](https://discord.gg/Ww9hbqr) to get involved and help us improve the SDK!
