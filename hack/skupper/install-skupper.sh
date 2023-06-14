#!/bin/bash
set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

if [ -z "$(which kubectl)" ]; then
		echo "Please install the kubectl command line tool first"
		echo "https://kubernetes.io/docs/tasks/tools/"
		exit 1
fi
if [ -z "$(which skupper)" ]; then
		echo "Please install the skupper command line tool first"
		echo "https://skupper.io/start/index.html#step-1-install-the-skupper-commandline-tool-in-your-environment"
		exit 1
fi

# Install MetalLB to get services of type LoadBalancer working (needed by Skupper)..
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml
kubectl wait --namespace metallb-system \
             --for=condition=ready pod \
             --selector=app=metallb \
             --timeout=90s
#
# Configure MetalLB
#
KIND_SUBNET=$(docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' kind)
echo "Your kind subnet is: ${KIND_SUBNET}"
# TODO: figure out how to carve out a small slice of the KIND_SUBNET to automatically set the ADDRESS_POOL
ADDRESS_POOL="172.18.255.200-172.18.255.250"
echo "Change ADDRESS_POOL setting in this script if ${ADDRESS_POOL} is not in your kind subnet"

cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - ${ADDRESS_POOL}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
EOF

kubectl create namespace west || true
kubectl config set-context --current --namespace west
skupper init --enable-console --enable-flow-collector
skupper token create ${SCRIPT_DIR}/west.token

kubectl create namespace east || true
kubectl config set-context --current --namespace east
skupper init
skupper link create ${SCRIPT_DIR}/west.token || true

# Verify the two sites are linked...
skupper status
kubectl config set-context --current --namespace west
skupper status

# Deploy some services for skupper to proxy
kubectl config set-context --current --namespace east
kubectl create deployment backend --image quay.io/skupper/hello-world-backend --replicas 3 || true
skupper expose deployment/backend --port 8080 || true

kubectl config set-context --current --namespace west
kubectl create deployment frontend --image quay.io/skupper/hello-world-frontend || true
kubectl expose deployment frontend --port 8080 --type LoadBalancer || true

# For some reason the LB ip:port did not work on my machine, so setup a port forward to
# access the front end app:
kubectl port-forward $(kubectl get pods -l app=frontend -o name) 8080&
sleep 2
open http://localhost:8080