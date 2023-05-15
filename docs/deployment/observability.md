# Monitoring

The Nexodus stack is designed to be observable.

## Monitoring Locally

```console
make deploy-monitoring-stack
```

### Accessing The Monitoring Stack

The dashboards for the services are available at:

- <http://grafana.127.0.0.1.nip.io>

## Monitoring in OpenShift

### Install Operators

The Prometheus Operator is not required since OpenShift Metrics provides the same functionality. Similarly, the Jaeger Operator is not required as the functionality is part of OpenShift Distributed Tracing.
