#!/bin/sh

set -e
info_message() {
    if [ -z "${1}" ]; then
        echo "info_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;34m ACTION \033[0m] ${1}"
}

pass_message() {
    if [ -z "${1}" ]; then
        echo "pass_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;32m PASSED \033[0m] ${1}"
}

error_message() {
    if [ -z "${1}" ]; then
        echo "error_message() requires a message"
        exit 1
    fi
    if [ -n "$1" ]; then
        echo -e "[\033[0;31m FAILED \033[0m] ] ${1}"
    fi
}

up() {
    if ! [ -x "$(command -v kind)" ]; then
        error_message "Kind is required. Please install Kind"
        exit 1
    fi

    if ! [ -x "$(command -v kubectl)" ]; then
        error_message "kubectl is required. Please install kubectl"
        exit 1
    fi

    if [ "$(command uname -s)" != "Linux" ]; then
        error_message "$(command basename $0) is only known to work on Linux (not $(command uname -s))"
        exit 1
    fi

    kind create cluster --config ./deploy/kind.yaml
    kubectl cluster-info --context kind-apex-dev

    trap "kubectl get pods -A" EXIT

    info_message "Deploying Ingress Controller"
    kubectl apply -f ./deploy/kind-ingress.yaml

    info_message "Installing Cert Manager"
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml

    info_message "Installing Postgres Operator"
    kubectl apply -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/namespace
    kubectl apply --server-side -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/default

    info_message "Creating Apex Namespace"
    kubectl create namespace apex

    info_message "Loading Images To KIND"
    kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
    kind load --name apex-dev docker-image quay.io/apex/frontend:latest

    # We need at least the nginx and cert-manager admission controllers running before Apex can be deployed.
    info_message "Waiting for System Readiness"
    kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=5m
    kubectl rollout status -n kube-system deployment coredns --timeout=5m
    kubectl rollout status -n cert-manager deploy/cert-manager --timeout=5m
    kubectl rollout status -n cert-manager deploy/cert-manager-webhook --timeout=5m
    kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=5m
    kubectl wait --for=condition=Ready pods --all -n postgres-operator --timeout=5m

    info_message "Deploying Apex"

    kubectl apply -k ./deploy/apex/overlays/dev

    info_message "Waiting for Apex Pod Readiness"
    kubectl wait --for=condition=Ready pods --all -n apex -l app.kubernetes.io/part-of=apex --timeout=15m

    if ! grep -q "apex.local" /etc/hosts ; then
        info_message "apex.local was not found in the /etc/hosts file. To access apex, update /etc/hosts by running: echo \"127.0.0.1 auth.apex.local api.apex.local apex.local\" | sudo tee -a /etc/hosts"
    fi
}

down() {
    kind delete cluster --name apex-dev
}

cacert() {
    if ! [ -x "$(command -v mkcert)" ]; then
        error_message "mkcert not found. Please install it from https://github.com/FiloSottile/mkcert"
        exit 1
    fi

    mkdir -p .certs
    kubectl get secret -n apex apex-ca-key-pair -o json | jq -r '.data."ca.crt"' | base64 -d > .certs/rootCA.pem
    CAROOT=$(pwd)/.certs mkcert -install
}

case $1 in
    "up")
        up
        ;;
    "down")
        down
        ;;
    "cacert")
        cacert
        ;;
    *)
        error_message "command required. up, down, or cacert"
        exit 1
        ;;
esac
