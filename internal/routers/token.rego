package token

import future.keywords

default valid_keycloak_token := false

default allowed_email := false

allowed_email if {
	token_payload.from_google
	endswith(token_payload.email, "redhat.com")
}

allowed_email if {
	not token_payload.from_google
}

valid_nexodus_token if {
	[valid, _, _] := io.jwt.decode_verify(input.access_token, {"cert": input.nexodus_jwks})
	valid == true
}

valid_keycloak_token if {
	[valid, _, _] := io.jwt.decode_verify(input.access_token, {"cert": input.jwks, "aud": "account"})
	valid == true
	allowed_email
}

valid_token if {
	valid_nexodus_token
}

valid_token if {
	valid_keycloak_token
}

default allow := false

allow if {
	"organizations" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"organizations" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	"vpcs" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"vpcs" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	"invitations" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"invitations" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	input.path[1] in ["devices", "sites"]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:devices")
}

allow if {
	input.path[1] in ["devices", "sites"]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:devices")
}

allow if {
	"users" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:users")
}

allow if {
	"users" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:users")
}

allow if {
	"security-groups" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"security-groups" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	"fflags" = input.path[1]
	valid_keycloak_token
}

allow if {
	"reg-keys" = input.path[1]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

allow if {
	"reg-keys" = input.path[1]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

# reg token can get its own token
allow if {
	valid_nexodus_token
	contains(token_payload.scope, "reg-token")
	action_is_read
	"reg-keys" = input.path[1]
	"me" = input.path[2]
}

# reg token can create a device or site
allow if {
	valid_nexodus_token
	contains(token_payload.scope, "reg-token")
	input.method == "POST"
	count(input.path) == 2
	input.path[1] in ["devices", "sites"]
}

# reg token can update a device or site
allow if {
	valid_nexodus_token
	contains(token_payload.scope, "reg-token")
	input.method == "PATCH"
	count(input.path) == 3
	input.path[1] in ["devices", "sites"]
}

# reg token can get a devices/orgs/vpcs/sites
allow if {
	valid_nexodus_token
	contains(token_payload.scope, "reg-token")
	input.method == "GET"
	input.path[1] in ["organizations", "vpcs", "devices", "sites"]
}

# device tokens can read/update a device
allow if {
	valid_nexodus_token
	contains(token_payload.scope, "device-token")
	input.method in ["GET", "PATCH"]
	input.path[1] in ["devices", "sites"]
}

allow if {
	input.path[1] in ["organizations", "vpcs"]
	action_is_read
	valid_nexodus_token
	contains(token_payload.scope, "device-token")
}

allow if {
	input.path[1] in ["organizations", "vpcs"]
	"events" = input.path[3]
	input.method == "POST"
	valid_nexodus_token
	contains(token_payload.scope, "device-token")
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
