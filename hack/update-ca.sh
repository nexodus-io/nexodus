#!/bin/sh
set -e

if [ -f /.certs/rootCA.pem ]; then
  if [ -x /usr/sbin/update-ca-certificates ]; then
    cp /.certs/rootCA.pem /usr/local/share/ca-certificates/cacerts.crt
    /usr/sbin/update-ca-certificates 2> /dev/null > /dev/null

    # the following is needed to get the playwright browsers to know about the root CA
    if [ -x /usr/bin/certutil ]; then
      mkdir -p $HOME/.pki/nssdb
      /usr/bin/certutil --empty-password -d $HOME/.pki/nssdb -N
      /usr/bin/certutil -d sql:$HOME/.pki/nssdb -A -t "C,," -n cacerts.crt -i /usr/local/share/ca-certificates/cacerts.crt
    fi

  elif [ -x /usr/bin/update-ca-trust ]; then
    cp ./.certs/rootCA.pem /etc/pki/ca-trust/source/anchors/rootCA.crt
    /usr/bin/update-ca-trust 2> /dev/null > /dev/null
  else
    echo "error: unable to add root CA certificate"
    exit 1
  fi
fi

if [ -n "$1" ] && [ "$1" != "prod" ]; then
  exec "$@"
fi

if [ "$1" = "prod" ]; then
  cat << EOF > ~/.bash_history
NEXD_LOGLEVEL=debug nexd --service-url https://qa.nexodus.io
NEXD_LOGLEVEL=debug nexd
nexd --service-url https://qa.nexodus.io
nexd
EOF

  cat << EOF > ~/.motd

To connect this container to the nexodus network, try running:

    nexd

Press the up arrow to get this command from bash history.

EOF

else
  cat << EOF > ~/.bash_history
NEXD_LOGLEVEL=debug nexd --service-url https://try.nexodus.io
NEXD_LOGLEVEL=debug nexd --service-url https://qa.nexodus.io
NEXD_LOGLEVEL=debug nexd --service-url https://try.nexodus.127.0.0.1.nip.io --reg-key RK:dev-admin-reg-key
nexd --service-url https://try.nexodus.io
nexd --service-url https://qa.nexodus.io
nexctl --service-url https://try.nexodus.127.0.0.1.nip.io --username admin --password floofykittens
nexd --service-url https://try.nexodus.127.0.0.1.nip.io --reg-key RK:dev-admin-reg-key
EOF

  cat << EOF > ~/.motd

To connect this container to the nexodus network, try running:

    nexd --service-url https://try.nexodus.127.0.0.1.nip.io --reg-key RK:dev-admin-reg-key

Commands for using a dev service, qa.nexodus.io, or try.nexodus.io are in bash history.

Press the up arrow to find these commands.

EOF
fi

tmux new-session -s nexd-session -d
tmux send-keys "cat ~/.motd" "C-m"
exec tmux attach-session -t nexd-session
