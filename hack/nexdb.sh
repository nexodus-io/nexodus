#!/bin/bash

set -e

# Run in the background, but kill it if this script exits
kubectl get pods -n nexodus | grep database-instance | grep Running | awk '{print $1}' | \
    xargs -I {} kubectl port-forward -n nexodus {} 55432:5432 &
PORT_FORWARD_PID=$!
echo ${PORT_FORWARD_PID}
trap "kill $PORT_FORWARD_PID" EXIT

DBPW=$(kubectl get secret -n nexodus database-pguser-apiserver -o json | jq -r '.data.password' | base64 -d)

echo "Password is: ${DBPW}"
docker run -it --rm --network=host jbergknoff/postgresql-client \
    --username apiserver --host localhost --port 55432 -d apiserver

# Note, to get to the ipam db, use these adjustments:
# kubectl get secret -n nexodus database-pguser-ipam -o json | jq -r '.data.password' | base64 -d
# psql -h 127.0.0.1 --username ipam -d ipam
