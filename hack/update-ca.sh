#!/bin/sh
set -e

if [ -f /.certs/rootCA.pem ]; then
  if [ -x /usr/sbin/update-ca-certificates ]; then
    cp /.certs/rootCA.pem /usr/local/share/ca-certificates/rootCA.crt
    /usr/sbin/update-ca-certificates 2> /dev/null > /dev/null
  elif [ -x /usr/bin/update-ca-trust ]; then
    cp ./.certs/rootCA.pem /etc/pki/ca-trust/source/anchors/rootCA.crt
    /usr/bin/update-ca-trust 2> /dev/null > /dev/null
  else
    echo "error: unable to add root CA certificate"
    exit 1
  fi
fi

if [ -z "$1" ]; then
  echo "To connect this container to the nexodus network, try running:"
  echo
  echo "   /bin/nexd --username admin --password floofykittens https://try.nexodus.127.0.0.1.nip.io"
  echo
  exec /bin/bash
else
  exec "$@"
fi