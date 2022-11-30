#!/bin/sh

set -e

up() {
    kind create cluster --config ./deploy/kind.yaml
    kubectl cluster-info --context kind-apex-dev

    # Deploy Ingress Controller
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=90s

    # Add rewrite to CoreDNS
    kubectl get -n kube-system cm/coredns -o yaml > coredns.yaml
    sed -i '22i \
            rewrite name auth.apex.local dex.apex.svc.cluster.local' coredns.yaml 
    kubectl replace -n kube-system -f coredns.yaml
    rm coredns.yaml
    kubectl rollout restart -n kube-system deployment/coredns
    kubectl rollout status -n kube-system deployment coredns --timeout=90s

    # Build images and copy to kind
    make images
    kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
    kind load --name apex-dev docker-image quay.io/apex/frontend:latest

    # Create namespace and deploy apex
    kubectl create namespace apex
    kubectl apply -k ./deploy/overlays/dev

    kubectl rollout status -n apex deployment dex --timeout=90s
    kubectl rollout status -n apex statefulset apiserver --timeout=90s
    kubectl rollout status -n apex statefulset ipam --timeout=90s
    kubectl rollout status -n apex deployment backend-web --timeout=90s
    kubectl rollout status -n apex deployment backend-cli --timeout=90s
    kubectl rollout status -n apex deployment apiproxy --timeout=90s
}

down() {
    kind delete cluster --name apex-dev
}

case $1 in
    "up")
        up
        ;;
    "down")
        down
        ;;
    *)
        echo "command required. up or down"
        exit 1
        ;;
esac
