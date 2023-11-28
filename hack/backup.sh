#!/bin/bash
set -e

POD=$(kubectl -n nexodus get pods -l postgres-operator.crunchydata.com/role=master -o name)

function backup() {
    kubectl -n nexodus exec -c database $POD -- pg_dump --blobs --clean --create --if-exists $1 > $1.sql
}

backup apiserver
backup ipam
backup keycloak