package token

import future.keywords

import input.path
import input.method
import input.jwks
import input.access_token

default allow := false

allow if {
	regex.match("^/organizations", input.path)
	action_is_read
	contains(claims.scope, "read:organizations")
}

allow if {
	regex.match("^/organizations", input.path)
	action_is_write
	contains(claims.scope, "write:organizations")
}

allow if {
	regex.match("^/devices", input.path)
	action_is_read
	contains(claims.scope, "read:devices")
}

allow if {
	regex.match("^/devices", input.path)
	action_is_write
	contains(claims.scope, "write:devices")
}

allow if {
	regex.match("^/users", input.path)
	action_is_read
	contains(claims.scope, "read:users")
}

allow if {
	regex.match("^/users", input.path)
	action_is_write
	contains(claims.scope, "write:users")
}

allow if {
	regex.match("^/fflags", input.path)
	claims
}

action_is_read if input.method in ["GET"]

action_is_write := input.method in ["POST", "PATCH", "DELETE"]

claims := payload if {
	[valid, payload, _] := io.jwt.decode_verify(input.access_token, {"cert": input.jwks})
	valid
}
