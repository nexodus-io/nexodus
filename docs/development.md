# Nexodus Development

- [Nexodus Development](#nexodus-development)
  - [Running the integration tests](#running-the-integration-tests)
    - [Prerequisites](#prerequisites)
    - [Using Docker](#using-docker)
    - [Using podman](#using-podman)

## Running the integration tests

### Prerequisites

Prior to running the integration tests, you must start the kind-based development environment. See the [Run on Kubernetes](nexodus-service.md#run-on-kubernetes) docs for further details.

### Using Docker

You can simply:

```console
make e2e
```

### Using podman

Since the test containers require CAP_NET_ADMIN only rootful podman can be used.
To run the tests requires a little more gymnastics.
This assumes you have `podman-docker` installed since testcontainers relies on mounting `/var/run/docker.sock` inside the reaper container.

```console
# Build test images in rootful podman
sudo make test-images
# Compile integration tests
go test -c --tags=integration ./integration-tests/...
# Run integration tests using rootful podman
sudo NEXODUS_TEST_PODMAN=1 TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED=true ./integration-tests.test -test.v
```
