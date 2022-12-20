#!/bin/sh

set -e

up() {
    kind create cluster --config ./deploy/kind.yaml
    kubectl cluster-info --context kind-apex-dev

    trap "kubectl get pods -A" EXIT

    echo "Deploying Ingress Controller"
    kubectl apply -f ./deploy/kind-ingress.yaml
    kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=5m

    echo "Adding Rewrite to CoreDNS"
    kubectl get -n kube-system cm/coredns -o yaml > coredns.yaml
    sed -i '22i \
            rewrite name auth.apex.local dex.apex.svc.cluster.local' coredns.yaml
    kubectl replace -n kube-system -f coredns.yaml
    rm coredns.yaml
    kubectl rollout restart -n kube-system deployment/coredns
    kubectl rollout status -n kube-system deployment coredns --timeout=5m

    echo "Installing Cert Manager"
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml
    kubectl rollout status -n cert-manager deploy/cert-manager --timeout=5m
    kubectl rollout status -n cert-manager deploy/cert-manager-webhook --timeout=5m
    kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=5m

    echo "Installing Postgres Operator"
    kubectl apply -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/namespace
    kubectl apply --server-side -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/default
    kubectl wait --for=condition=Ready pods --all -n postgres-operator --timeout=5m
    
    echo "Loading Images To KIND"
    kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
    kind load --name apex-dev docker-image quay.io/apex/frontend:latest

    echo "Deploying Apex"
    kubectl create namespace apex
    kubectl apply -k ./deploy/apex/overlays/dev

    echo "Waiting for Apex Pod Readiness"
    kubectl wait --for=condition=Ready pods --all -n apex -l app.kubernetes.io/part-of=apex --timeout=15m
}

down() {
    kind delete cluster --name apex-dev
}

cacert() {
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
        echo "command required. up, down, or cacert"
        exit 1
        ;;
esac
