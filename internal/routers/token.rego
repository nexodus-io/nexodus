package token

import future.keywords

default valid_token := false

valid_token if {
	[valid, _, _] := io.jwt.decode_verify(input.access_token, {"cert": input.jwks, "aud": "account"})
	valid == true
}

default allow := false

default org_id := false

org_id = input.path[2]

default user_in_org := false

user_in_org if not org_id

user_in_org if data.user_org_map[user_id][org_id]

default user_is_self := false

user_is_self if "me" = input.path[2]

user_is_self if user_id == input.path[2]

default organizations := []

organizations := data.user_org_map[user_id]

allow if {
	"organizations" = input.path[1]
	action_is_read
	valid_token
	contains(token_payload.scope, "read:organizations")
	user_in_org
}

allow if {
	"organizations" = input.path[1]
	action_is_write
	valid_token
	contains(token_payload.scope, "write:organizations")
	user_in_org
}

allow if {
	"invitations" = input.path[1]
	action_is_read
	valid_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"invitations" = input.path[1]
	action_is_write
	valid_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	"devices" = input.path[1]
	action_is_read
	valid_token
	contains(token_payload.scope, "read:devices")
}

allow if {
	"devices" = input.path[1]
	action_is_write
	valid_token
	contains(token_payload.scope, "write:devices")
}

allow if {
	"users" = input.path[1]
	action_is_read
	valid_token
	contains(token_payload.scope, "read:users")
	user_is_self
}

allow if {
	"users" = input.path[1]
	action_is_write
	valid_token
	contains(token_payload.scope, "write:users")
	user_is_self
}

allow if {
	"fflags" = input.path[1]
	valid_token
}

action_is_read if input.method in ["GET"]

action_is_write := input.method in ["POST", "PATCH", "DELETE", "PUT"]

token_payload := payload if {
	[_, payload, _] = io.jwt.decode(input.access_token)
}

default user_id = ""

user_id = token_payload.sub

default user_name = ""

user_name = token_payload.preferred_username

default full_name = ""

full_name = token_payload.name
