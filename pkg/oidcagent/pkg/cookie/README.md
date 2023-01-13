# Cookie store for [Session](https://github.com/go-session/session)

[![Build][Build-Status-Image]][Build-Status-Url] [![Codecov][codecov-image]][codecov-url] [![ReportCard][reportcard-image]][reportcard-url] [![GoDoc][godoc-image]][godoc-url] [![License][license-image]][license-url]

## Quick Start

### Download and install

```bash
$ go get -u -v github.com/go-session/cookie
```

### Create file `server.go`

```go
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-session/cookie"
	"github.com/go-session/session"
)

var (
	hashKey = []byte("FF51A553-72FC-478B-9AEF-93D6F506DE91")
)

func main() {
	session.InitManager(
		session.SetStore(
			cookie.NewCookieStore(
				cookie.SetCookieName("demo_cookie_store_id"),
				cookie.SetHashKey(hashKey),
			),
		),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		store, err := session.Start(context.Background(), w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}

		store.Set("foo", "bar")
		err = store.Save()
		if err != nil {
			fmt.Fprint(w, err)
			return
		}

		http.Redirect(w, r, "/foo", 302)
	})

	http.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		store, err := session.Start(context.Background(), w, r)
		if err != nil {
			fmt.Fprint(w, err)
			return
		}

		foo, ok := store.Get("foo")
		if !ok {
			fmt.Fprint(w, "does not exist")
			return
		}

		fmt.Fprintf(w, "foo:%s", foo)
	})

	http.ListenAndServe(":8080", nil)
}
```

### Build and run

```bash
$ go build server.go
$ ./server
```

### Open in your web browser

<http://localhost:8080>

    foo:bar


## MIT License

    Copyright (c) 2018 Lyric

[Build-Status-Url]: https://travis-ci.org/go-session/cookie
[Build-Status-Image]: https://travis-ci.org/go-session/cookie.svg?branch=master
[codecov-url]: https://codecov.io/gh/go-session/cookie
[codecov-image]: https://codecov.io/gh/go-session/cookie/branch/master/graph/badge.svg
[reportcard-url]: https://goreportcard.com/report/github.com/go-session/cookie
[reportcard-image]: https://goreportcard.com/badge/github.com/go-session/cookie
[godoc-url]: https://godoc.org/github.com/go-session/cookie
[godoc-image]: https://godoc.org/github.com/go-session/cookie?status.svg
[license-url]: http://opensource.org/licenses/MIT
[license-image]: https://img.shields.io/npm/l/express.svg
