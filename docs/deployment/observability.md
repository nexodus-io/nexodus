# Monitoring

The Nexodus stack is designed to be observable.

## Monitoring Locally

### Install Operators

The monitoring stack requires several operators.
To install all of the following components:

- Prometheus Operator
- Jaeger Operator
- Grafana Operator

```console
kubectl create -k ./deploy/observability-operators/base
kubectl wait --for=condition=Ready pods --all -n observability --timeout=300s
```

### Install Monitoring Stack

```console
kubectl create namespace nexodus-monitoring
kubectl apply -k ./deploy/nexodus-monitoring/overlays/dev
```

### Accessing The Monitoring Stack

The dashboards for the services are available at:

- <http://jaeger.127.0.0.1.nip.io>
- <http://prometheus.127.0.0.1.nip.io>
- <http://grafana.127.0.0.1.nip.io>

## Monitoring in OpenShift

### Install Operators

The Prometheus Operator is not required since OpenShift Metrics provides the same functionality. Similarly, the Jaeger Operator is not required as the functionality is part of OpenShift Distributed Tracing.
