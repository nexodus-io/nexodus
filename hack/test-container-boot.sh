#!/bin/sh

if [ -f /.certs/rootCA.pem ]; then
  if [ -x /usr/sbin/update-ca-certificates ]; then
    cp /.certs/rootCA.pem /usr/local/share/ca-certificates/rootCA.crt
    /usr/sbin/update-ca-certificates
  elif [ -x /usr/bin/update-ca-trust ]; then
    cp ./.certs/rootCA.pem /etc/pki/ca-trust/source/anchors/rootCA.crt
    /usr/bin/update-ca-trust
  else
    echo "error: unable to add root CA certificate"
    exit 1
  fi
fi
if [ -n "$1" ]; then
  exec $*
fi
echo "To connect this container to the apex network, try running:"
echo
echo "   /bin/apexd --username admin --password floofykittens https://apex.local"
echo

