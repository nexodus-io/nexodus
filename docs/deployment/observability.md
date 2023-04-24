# Monitoring

The Nexodus stack is designed to be observable.

## Monitoring Locally

To install all of the following components:

- Prometheus Operator
- Jaeger Operator
- And the necessary configuration for apiserver metrics/traces to export to instances of Prometheus/Jaeger managed by the operator
- Grafana Operator
- Grafana datastore (pointed at Prometheus)
- Grafana dashboards

```console
kubectl create -k ./deploy/observability-operators/base
kubectl wait --for=condition=Ready pods --all -n nexodus-monitoring --timeout=300s
```

Uncomment the commented lines in `./deploy/nexodus/overlays/dev/kustomization.yaml` then run:

```console
kubectl apply -k ./deploy/nexodus/overlays/dev
```

The dashboards for the services are available at:

- <http://jaeger.127.0.0.1.nip.io>
- <http://prometheus.127.0.0.1.nip.io>
- <http://grafana.127.0.0.1.nip.io>
