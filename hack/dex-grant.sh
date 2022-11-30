#!/bin/sh

set -e

source $(pwd)/.env

USERNAME="$1"
PASSWORD="$2"

token=$(curl -s -f -X POST \
    $APEX_OIDC_URL/token \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d "username=$USERNAME" \
    -d "password=$PASSWORD" \
    -d "scope=openid profile email" \
    -d "grant_type=password" \
    -d "client_id=$APEX_OIDC_CLIENT_ID_CLI"
)

echo $token
