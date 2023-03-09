package cookie

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-session/session/v3"
)

var (
	hashKey = []byte("FF51A553-72FC-478B-9AEF-93D6F506DE91")
)

func TestCookie(t *testing.T) {
	sess := session.NewManager(
		session.SetCookieName("test_cookie"),
		session.SetSign([]byte("sign")),
		session.SetStore(NewCookieStore(
			SetCookieName("test_cookie_store"),
			SetHashKey(hashKey),
		)),
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store, err := sess.Start(context.Background(), w, r)
		if err != nil {
			t.Error(err)
			return
		}

		if r.URL.Query().Get("login") == "1" {
			foo, ok := store.Get("foo")
			fmt.Fprintf(w, "%s:%v", foo, ok)
			return
		}

		store.Set("foo", "bar")
		err = store.Save()
		if err != nil {
			t.Error(err)
			return
		}

		fmt.Fprint(w, "ok")
	}))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Error(err)
		return
	}

	buf, _ := io.ReadAll(res.Body)
	if string(buf) != "ok" {
		t.Error("Not expected value:", string(buf))
		return
	}
	res.Body.Close()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?login=1", ts.URL), nil)
	if err != nil {
		t.Error(err)
		return
	}

	for _, c := range res.Cookies() {
		req.AddCookie(c)
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}

	buf, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if string(buf) != "bar:true" {
		t.Error("Not expected value:", string(buf))
		return
	}
}
