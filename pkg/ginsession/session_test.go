package ginsession

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-session/session/v3"
)

func TestSession(t *testing.T) {
	cookieName := "test_gin_session"

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(New(
		session.SetCookieName(cookieName),
		session.SetSign([]byte("sign")),
	))

	r.Use(func(ctx *gin.Context) {
		store := FromContext(ctx)
		if ctx.Query("login") == "1" {
			foo, ok := store.Get("foo")
			fmt.Fprintf(ctx.Writer, "%s:%v", foo, ok)
			return
		}

		store.Set("foo", "bar")
		err := store.Save()
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprint(ctx.Writer, "ok")
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Error(err)
		return
	}
	r.ServeHTTP(w, req)

	res := w.Result()
	cookie := res.Cookies()[0]
	if cookie.Name != cookieName {
		t.Error("Not expected value:", cookie.Name)
		return
	}

	buf, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(buf) != "ok" {
		t.Error("Not expected value:", string(buf))
		return
	}

	req, err = http.NewRequest("GET", "/?login=1", nil)
	if err != nil {
		t.Error(err)
		return
	}
	req.AddCookie(cookie)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res = w.Result()
	buf, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if string(buf) != "bar:true" {
		t.Error("Not expected value:", string(buf))
		return
	}
}
