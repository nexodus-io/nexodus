#!/bin/bash
set -e

kubectl config set-context --current --namespace west
make deploy-nexd-router
kubectl scale deployment skupper-router --replicas 0
kubectl rollout status deployment skupper-router --timeout=1m

kubectl config set-context --current --namespace east
make deploy-nexd-router
kubectl scale deployment skupper-router --replicas 0
kubectl rollout status deployment skupper-router --timeout=1m

kubectl -n west port-forward $(kubectl -n west  get pods -l app=frontend -o name) 8080 &
sleep 2
open http://localhost:8080
