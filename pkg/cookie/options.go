package cookie

import (
	"crypto/cipher"
	"hash"
)

// Define default options
var defaultOptions = options{
	cookieName: "cookie_session_id",
	maxLength:  -1,
	maxAge:     -1,
	minAge:     -1,
}

type options struct {
	hashFunc   func() hash.Hash
	blockFunc  func([]byte) (cipher.Block, error)
	cookieName string
	hashKey    []byte
	blockKey   []byte
	maxLength  int
	maxAge     int
	minAge     int
	secure     bool
}

// Option A cookie parameter options
type Option func(*options)

// SetCookieName Set the cookie name
func SetCookieName(cookieName string) Option {
	return func(o *options) {
		o.cookieName = cookieName
	}
}

// SetSecure Set cookie security
func SetSecure(secure bool) Option {
	return func(o *options) {
		o.secure = secure
	}
}

// SetHashKey used to authenticate values using HMAC
func SetHashKey(hashKey []byte) Option {
	return func(o *options) {
		o.hashKey = hashKey
	}
}

// SetHashFunc sets the hash function used to create HMAC
func SetHashFunc(hashFunc func() hash.Hash) Option {
	return func(o *options) {
		o.hashFunc = hashFunc
	}
}

// SetBlockKey used to encrypt values
func SetBlockKey(blockKey []byte) Option {
	return func(o *options) {
		o.blockKey = blockKey
	}
}

// SetBlockFunc sets the encryption function used to create a cipher.Block
func SetBlockFunc(blockFunc func([]byte) (cipher.Block, error)) Option {
	return func(o *options) {
		o.blockFunc = blockFunc
	}
}

// SetMaxLength restricts the maximum length, in bytes, for the cookie value
func SetMaxLength(maxLength int) Option {
	return func(o *options) {
		o.maxLength = maxLength
	}
}

// SetMaxAge restricts the maximum age, in seconds, for the cookie value
func SetMaxAge(maxAge int) Option {
	return func(o *options) {
		o.maxAge = maxAge
	}
}

// SetMinAge restricts the minimum age, in seconds, for the cookie value
func SetMinAge(minAge int) Option {
	return func(o *options) {
		o.minAge = minAge
	}
}
