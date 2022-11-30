Deploy on Kind
==============


## Create Cluster

```console
kind create cluster ./deploy/kind.yaml
kubectl cluster-info --context kind-apex-dev
```

## Install Ingress Controller

```console
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=90s
```

## Fix internal name resolution

```console
kubectl edit configmap -n kube-system coredns
```

Add the following line in `:53{}`
```
rewrite name auth.apex.local dex.apex.svc.cluster.local
```

Then restart core-dns:
```console
kubectl rollout restart -n kube-system deployment/coredns
kubectl rollout status -n kube-system deployment coredns
```

## Load Images

```console
make images
kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
kind load --name apex-dev docker-image quay.io/apex/frontend:latest
```

## Install Apex

```console
kubectl create namespace apex
kubectl apply -k ./deploy/overlays/dev
kubectl rollout status -n apex statefulset ipam
```
