#!/bin/sh

set -e

up() {
    kind create cluster --config ./deploy/kind.yaml
    kubectl cluster-info --context kind-apex-dev

    echo "Deploying Ingress Controller"
    kubectl apply -f ./deploy/kind-ingress.yaml
    kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=300s

    echo "Adding Rewrite to CoreDNS"
    kubectl get -n kube-system cm/coredns -o yaml > coredns.yaml
    sed -i '22i \
            rewrite name auth.apex.local dex.apex.svc.cluster.local' coredns.yaml
    kubectl replace -n kube-system -f coredns.yaml
    rm coredns.yaml
    kubectl rollout restart -n kube-system deployment/coredns
    kubectl rollout status -n kube-system deployment coredns --timeout=300s

    echo "Installing Cert Manager"
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml
    kubectl rollout status -n cert-manager deploy/cert-manager --timeout=300s
    kubectl rollout status -n cert-manager deploy/cert-manager-webhook --timeout=300s
    kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=300s

    echo "Loading Images To KIND"
    kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
    kind load --name apex-dev docker-image quay.io/apex/frontend:latest

    echo "Deploying Apex"
    kubectl create namespace apex
    kubectl apply -k ./deploy/apex/overlays/dev

    kubectl rollout status -n apex deployment dex --timeout=300s
    kubectl rollout status -n apex deployment dex --timeout=300s
    kubectl rollout status -n apex statefulset ipam --timeout=300s
    kubectl rollout status -n apex statefulset apiserver --timeout=300s
    kubectl rollout status -n apex deployment backend-web --timeout=300s
    kubectl rollout status -n apex deployment backend-cli --timeout=300s
    kubectl rollout status -n apex deployment apiproxy --timeout=300s

    echo "Waiting for Apex Pod Readiness"
    kubectl wait --for=condition=Ready pods --all -n apex --timeout=20m

    echo "Waiting 5s until we add healthecks/liveness probes"
    # give k8s a little longer to come up
    sleep 5
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
