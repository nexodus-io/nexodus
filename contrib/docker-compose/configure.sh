#!/bin/bash
set -e
cd $(dirname "$0")

if [[ ! -f .env  || "${1}" == "--force" ]] ; then
  cp example.env .env
fi

set -o allexport
source .env
set +o allexport

if [[ ! -d ./volumes/ingress/certs || "${1}" == "--force" ]] ; then
  mkdir -p ../../certs || true
  mkdir -p ./volumes/ingress/certs || true
  CAROOT=../../certs mkcert -install \
    -cert-file ./volumes/ingress/certs/tls.crt \
    -key-file  ./volumes/ingress/certs/tls.key \
    ${APIPROXY_WEB_DOMAIN} \
    ${APIPROXY_AUTH_DOMAIN} \
    ${APIPROXY_API_DOMAIN}
fi

cat > .env-keys <<EOF
NEXAPI_TLS_KEY="$(cat ./volumes/ingress/certs/tls.key)"
EOF

mkdir -p ./volumes/envoy/sockets || true
mkdir -p ./volumes/envoy/config || true
mkdir -p ./volumes/apiserver/sockets || true

cp ../../deploy/nexodus/base/apiproxy/files/*  ./volumes/envoy/config
cat ../../deploy/nexodus/base/apiproxy/files/envoy.yaml | \
  envsubst > ./volumes/envoy/config/envoy.yaml

echo "Done.... now run:"
echo
echo "   docker-compose up -d"
echo
