#!/bin/sh

HOST="localhost:8888"
REALM="controller"
USERNAME="$1"
PASSWORD="$2"
CLIENTID='api-clients'
CLIENTSECRET='cvXhCRXI2Vld244jjDcnABCMrTEq2rwE'

curl -s -X POST \
    http://$HOST/realms/$REALM/protocol/openid-connect/token \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d "username=$USERNAME" \
    -d "password=$PASSWORD" \
    -d "grant_type=password" \
    -d "client_id=$CLIENTID" \
    -d "client_secret=$CLIENTSECRET" | jq -r ".access_token"
