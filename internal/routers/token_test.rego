package test_token

import data.token
import future.keywords

valid_user(scopes) := s if {
	s = {
		"sub": "00a7b7f4-f11f-4ea3-89de-7b1cde4316a9",
		"scope": scopes,
		"name": "valid-user",
		"full_name": "Valid User",
	}
	s
}

mock_decode_verify("org-write-jwt", _) := [true, {}, {}]

mock_decode("org-write-jwt") := [{}, valid_user("openid profile email write:organizations"), {}]

mock_decode_verify("org-read-jwt", _) := [true, {}, {}]

mock_decode("org-read-jwt") := [{}, valid_user("openid profile email read:organizations"), {}]

mock_decode_verify("device-write-jwt", _) := [true, {}, {}]

mock_decode("device-write-jwt") := [{}, valid_user("openid profile email write:devices"), {}]

mock_decode_verify("device-read-jwt", _) := [true, {}, {}]

mock_decode("device-read-jwt") := [{}, valid_user("openid profile email read:devices"), {}]

mock_decode_verify("user-write-jwt", _) := [true, {}, {}]

mock_decode("user-write-jwt") := [{}, valid_user("openid profile email write:users"), {}]

mock_decode_verify("user-read-jwt", _) := [true, {}, {}]

mock_decode("user-read-jwt") := [{}, valid_user("openid profile email read:users"), {}]

mock_decode_verify("bad-jwt", _) := [false, {}, {}]

test_org_get_allowed if {
	token.allow with input.path as ["api", "organizations"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "org-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_org_get_member if {
	token.allow with input.path as ["api", "organizations", "foo"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "org-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_org_post_allowed if {
	token.allow with input.path as ["api", "organizations"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "org-write-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_org_post_with_read_scope_denied if {
	not token.allow with input.path as ["api", "organizations"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "org-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_org_get_anonymous_denied if {
	not token.allow with input.path as ["api", "organizations"]
		with input.method as "GET"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_org_get_bad_jwt_denied if {
	not token.allow with input.path as ["api", "organizations"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_device_get_allowed if {
	token.allow with input.path as ["api", "devices"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "device-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_device_post_allowed if {
	token.allow with input.path as ["api", "devices"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "device-write-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_device_post_with_read_scope_denied if {
	not token.allow with input.path as ["api", "devices"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "device-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_device_get_anonymous_denied if {
	not token.allow with input.path as ["api", "devices"]
		with input.method as "GET"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_device_get_bad_jwt_denied if {
	not token.allow with input.path as ["api", "devices"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "bad-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_user_get_allowed if {
	token.allow with input.path as ["api", "users", "me"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "user-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_user_post_allowed if {
	token.allow with input.path as ["api", "users", "me"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "user-write-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_user_post_with_read_scope_denied if {
	not token.allow with input.path as ["api", "users", "me"]
		with input.method as "POST"
		with input.jwks as "my-cert"
		with input.access_token as "user-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_user_get_anonymous_denied if {
	not token.allow with input.path as ["api", "users", "me"]
		with input.method as "GET"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_user_get_bad_jwt_denied if {
	not token.allow with input.path as ["api", "users", "me"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "bad-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_get_fflags if {
	token.allow with input.path as ["api", "fflags"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "user-read-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_get_fflags_anonymous_denied if {
	not token.allow with input.path as ["api", "fflags"]
		with input.method as "GET"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}

test_get_fflags_bad_jwt_denied if {
	not token.allow with input.path as ["api", "fflags"]
		with input.method as "GET"
		with input.jwks as "my-cert"
		with input.access_token as "bad-jwt"
		with io.jwt.decode_verify as mock_decode_verify
		with io.jwt.decode as mock_decode
}
