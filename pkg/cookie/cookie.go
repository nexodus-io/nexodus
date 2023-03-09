package cookie

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-session/session/v3"
	"github.com/gorilla/securecookie"
)

var (
	_ session.ManagerStore = &managerStore{}
	_ session.Store        = &store{}
)

// NewCookieStore Create an instance of a cookie store
func NewCookieStore(opt ...Option) session.ManagerStore {
	opts := defaultOptions
	for _, o := range opt {
		o(&opts)
	}

	cookie := securecookie.New(opts.hashKey, opts.blockKey)
	if v := opts.hashFunc; v != nil {
		cookie = cookie.HashFunc(v)
	}
	if v := opts.blockFunc; v != nil {
		cookie = cookie.BlockFunc(v)
	}
	if v := opts.maxLength; v != -1 {
		cookie = cookie.MaxLength(v)
	}
	if v := opts.maxAge; v != -1 {
		cookie = cookie.MaxAge(v)
	}
	if v := opts.minAge; v != -1 {
		cookie = cookie.MinAge(v)
	}

	return &managerStore{
		opts:   opts,
		cookie: cookie,
	}
}

type managerStore struct {
	cookie *securecookie.SecureCookie
	opts   options
}

func (s *managerStore) Create(ctx context.Context, sid string, expired int64) (session.Store, error) {
	return newStore(ctx, s, sid, expired, nil), nil
}

func (s *managerStore) Update(ctx context.Context, sid string, expired int64) (session.Store, error) {
	req, ok := session.FromReqContext(ctx)
	if !ok {
		return nil, nil
	}

	cookie, err := req.Cookie(s.opts.cookieName)
	if err != nil {
		return newStore(ctx, s, sid, expired, nil), nil
	}

	res, ok := session.FromResContext(ctx)
	if !ok {
		return nil, nil
	}
	cookie.Expires = time.Now().Add(time.Duration(expired) * time.Second)
	cookie.MaxAge = int(expired)
	http.SetCookie(res, cookie)

	var values map[string]interface{}
	err = s.cookie.Decode(sid, cookie.Value, &values)
	if err != nil {
		return nil, err
	}

	return newStore(ctx, s, sid, expired, values), nil
}

func (s *managerStore) Delete(ctx context.Context, sid string) error {
	exists, err := s.Check(ctx, sid)
	if err != nil {
		return err
	} else if !exists {
		return nil
	}

	res, ok := session.FromResContext(ctx)
	if !ok {
		return nil
	}
	cookie := &http.Cookie{
		Name:     s.opts.cookieName,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now(),
		MaxAge:   -1,
	}
	http.SetCookie(res, cookie)

	return nil
}

func (s *managerStore) Check(ctx context.Context, sid string) (bool, error) {
	req, ok := session.FromReqContext(ctx)
	if !ok {
		return false, nil
	}

	_, err := req.Cookie(s.opts.cookieName)
	return err == nil, nil
}

func (s *managerStore) Refresh(ctx context.Context, oldsid, sid string, expired int64) (session.Store, error) {
	req, ok := session.FromReqContext(ctx)
	if !ok {
		return nil, nil
	}

	cookie, err := req.Cookie(s.opts.cookieName)
	if err != nil {
		return newStore(ctx, s, sid, expired, nil), nil
	}

	var values map[string]interface{}
	err = s.cookie.Decode(oldsid, cookie.Value, &values)
	if err != nil {
		return nil, err
	}

	encoded, err := s.cookie.Encode(sid, values)
	if err != nil {
		return nil, err
	}

	cookie.Value = encoded
	cookie.Expires = time.Now().Add(time.Duration(expired) * time.Second)
	cookie.MaxAge = int(expired)
	res, ok := session.FromResContext(ctx)
	if !ok {
		return nil, nil
	}
	http.SetCookie(res, cookie)

	return newStore(ctx, s, sid, expired, values), nil
}

func (s *managerStore) Close() error {
	return nil
}

func newStore(ctx context.Context, s *managerStore, sid string, expired int64, values map[string]interface{}) *store {
	if values == nil {
		values = make(map[string]interface{})
	}

	return &store{
		opts:    s.opts,
		cookie:  s.cookie,
		ctx:     ctx,
		sid:     sid,
		expired: expired,
		values:  values,
	}
}

type store struct {
	sync.RWMutex
	ctx     context.Context
	cookie  *securecookie.SecureCookie
	values  map[string]interface{}
	sid     string
	opts    options
	expired int64
}

func (s *store) Context() context.Context {
	return s.ctx
}

func (s *store) SessionID() string {
	return s.sid
}

func (s *store) Set(key string, value interface{}) {
	s.Lock()
	s.values[key] = value
	s.Unlock()
}

func (s *store) Get(key string) (interface{}, bool) {
	s.RLock()
	val, ok := s.values[key]
	s.RUnlock()
	return val, ok
}

func (s *store) Delete(key string) interface{} {
	s.RLock()
	v, ok := s.values[key]
	s.RUnlock()
	if ok {
		s.Lock()
		delete(s.values, key)
		s.Unlock()
	}
	return v
}

func (s *store) Flush() error {
	s.Lock()
	s.values = make(map[string]interface{})
	s.Unlock()
	return s.Save()
}

func (s *store) Save() error {
	s.RLock()
	encoded, err := s.cookie.Encode(s.sid, s.values)
	if err != nil {
		s.RUnlock()
		return err
	}
	s.RUnlock()

	cookie := &http.Cookie{
		Name:     s.opts.cookieName,
		Value:    encoded,
		Path:     "/",
		Secure:   s.opts.secure,
		HttpOnly: true,
		MaxAge:   int(s.expired),
		Expires:  time.Now().Add(time.Duration(s.expired) * time.Second),
	}

	res, ok := session.FromResContext(s.Context())
	if !ok {
		return nil
	}

	http.SetCookie(res, cookie)
	return nil
}
