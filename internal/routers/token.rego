package token

import future.keywords

default valid_keycloak_token := false

valid_nexodus_token if {
	[valid, _, _] := io.jwt.decode_verify(input.access_token, {"cert": input.nexodus_jwks})
	valid == true
}

valid_keycloak_token if {
	[valid, _, _] := io.jwt.decode_verify(input.access_token, {"cert": input.jwks, "aud": "account"})
	valid == true
}

valid_token if {
	valid_nexodus_token
}

valid_token if {
	valid_keycloak_token
}

valid_reg_token if {
	valid_nexodus_token
	contains(token_payload.scope, "reg-token")
}

valid_device_token if {
	valid_nexodus_token
	contains(token_payload.scope, "device-token")
}

default allow := false

allow if {
	input.path[1] in [
		"organizations",
		"invitations",
		"reg-keys",
		"vpcs",
		"service-networks",
		"security-groups",
	]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:organizations")
}

##

allow if {
	input.path[1] in [
		"organizations",
		"invitations",
		"reg-keys",
		"vpcs",
		"service-networks",
		"security-groups",
	]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:organizations")
}

allow if {
	input.path[1] in [
		"devices",
		"sites",
	]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:devices")
}

allow if {
	input.path[1] in [
		"devices",
		"sites",
	]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:devices")
}

allow if {
	input.path[1] in ["users"]
	action_is_read
	valid_keycloak_token
	contains(token_payload.scope, "read:users")
}

allow if {
	input.path[1] in ["users"]
	action_is_write
	valid_keycloak_token
	contains(token_payload.scope, "write:users")
}

allow if {
	input.path[1] in ["fflags"]
	valid_keycloak_token
}

# reg token can get its own token
allow if {
	"reg-keys" = input.path[1]
	"me" = input.path[2]
	action_is_read
	valid_reg_token
}

# reg token can create a device or site
allow if {
	count(input.path) == 2
	input.path[1] in [
		"devices",
		"sites",
	]
	input.method == "POST"
	valid_reg_token
}

# reg token can update a device or site
allow if {
	count(input.path) == 3
	input.path[1] in [
		"devices",
		"sites",
	]
	input.method == "PATCH"
	valid_reg_token
}

# reg token can update a device metadata
allow if {
	count(input.path) == 5
	"devices" = input.path[1]
	"metadata" = input.path[3]
	input.method == "PUT"
	valid_reg_token
}

# reg token can get a devices/orgs/vpcs/sites/service-networks
allow if {
	input.path[1] in [
		"organizations",
		"vpcs",
		"devices",
		"sites",
		"service-networks",
	]
	action_is_read
	valid_reg_token
}

# device tokens can read/update a device
allow if {
	input.path[1] in [
		"devices",
		"sites",
	]
	input.method in ["GET", "PATCH"]
	valid_device_token
}

allow if {
	input.path[1] in [
		"organizations",
		"vpcs",
		"service-networks",
	]
	action_is_read
	valid_device_token
}

allow if {
	input.path[1] in ["vpcs"]
	"events" = input.path[3]
	input.method == "POST"
	valid_device_token
}

allow if {
	input.path == ["api", "events"]
	input.method == "POST"
	valid_device_token
}

allow if {
	input.path == ["api", "events"]
	input.method == "POST"
	valid_keycloak_token
}

allow if {
	"ca" = input.path[1]
	valid_token
	# contains(token_payload.scope, "read:users")
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
