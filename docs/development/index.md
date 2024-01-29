# Nexodus Development

This is the development documentation for Nexodus. It is intended for developers who want to contribute to Nexodus.

## Getting Started

### Prerequisites

Linux and Mac environments are supported for development.

Development of Nexodus requires the following dependencies:

- Docker
- Go

Other dependencies can be installed by running `hack/install-tools.sh`. See
[the script](https://github.com/nexodus-io/nexodus/blob/main/hack/install-tools.sh)
for further details on the dependencies it installs.

### Building

Run `make` to see documentation of the available build targets. The most common build
target is `make all`, which will build the agent, API server, and supporting utilities.
It will also build them across supported platforms and architectures.

### More Details

For further details on the development environment, including IDE-specific tips,
see [Development Tooling](tooling.md).

## Testing your changes

For some changes to `nexd` or `nexctl`, you can run them against our
[production](https://try.nexodus.io) or [QA](https://qa.nexodus.io) environments.

For changes where you want to run your own local copy of the whole Nexodus service stack,
we offer a development environment that runs in [kind](https://kind.sigs.k8s.io/). To start
this environment, run `make run-on-kind`. This will start a local Kubernetes cluster and deploy
the Nexodus service stack to it. You can then run `make run-nexd-container` to quickly start an
instance of the agent and connect it to the local instance of Nexodus.

For more details on this environment, see the [Nexodus Service](../deployment/nexodus-service.md)
documentation.

### Automated Tests

Unit tests are run with `make test`. Integration tests are run against a local Nexodus instance
running in `kind` with `make e2e`. Further details on the integration tests can be found in
[the integration tests documentation](integration-tests.md).

## Debugging

Debugging tips can be found in [Debugging](debugging.md).

## Contributing

See the [pull requests](pull-requests.md) page for details on how to contribute to Nexodus.

Take a look at the project's [guiding principles](development.md) to get a sense of some of
the factors that influence the project's direction.