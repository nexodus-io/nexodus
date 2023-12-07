#!/bin/bash
set -e

if [ $# -eq 0 ];   then
  echo "usage: $0 <ca-domain-name>"
  echo "example: $0 try.nexodus.io"
  exit 1
fi

if [ -d ./.ca ]; then
  echo "error: ./ca directory already exists"
  exit 1
fi

echo "Generating new CA"
mkdir ./.ca || true
chmod 700 ./.ca
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out ./.ca/tls.key
openssl req -new -x509 -days 3650 -key ./.ca/key.pem -out ./.ca/ca.crt -subj "/CN=$1"

touch apiserver-ca.yaml
chmod 600 apiserver-ca.yaml
kubectl create secret generic apiserver-ca \
    --from-file=ca.crt=./.ca/ca.crt \
    --from-file=tls.key=./.ca/tls.key \
    --output yaml --dry-run=client > nexodus-ca-key-pair.yaml
echo "Created nexodus-ca-key-pair.yaml"
rm -rf ./.ca
