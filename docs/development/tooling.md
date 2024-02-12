# Nexodus Development Tooling

Tools and tips that will aid you in the development and testing of Nexodus on your local machine.

## Tools

### Base

* [git](https://git-scm.com/) our source control tool
* [Go](https://go.dev/dl/) v1.19 or v1.20
* make - our main build tool
* bash - we use a few bash scripts to set things up.
* [docker](https://www.docker.com) or [podman](https://podman.io/)/[buildah](https://buildah.io/) - to build and run containers
* [kubectl](https://kubernetes.io/docs/tasks/tools/) - to interact with your kube deployments
* [jq](https://stedolan.github.io/jq/) - A lightweight and flexible command-line JSON processor

### Go Based Build Tools

The following tools can be installed with by running:

    ./hack/install-tools.sh [--force]

Use the `--force` option if you want to reinstall them in case you think the versions your using are not right.

* [kind](https://kind.sigs.k8s.io/) - used to spin up a small kube cluster for testing.
* [golangci-lint](https://golangci-lint.run/) - go language linters
* [swag](github.com/swaggo/swag) - code first openapi spec document generator.
* [mkcert](https://github.com/FiloSottile/mkcert) - to generate and install ca certificates.

## IDE Tips

### [VSCode](https://code.visualstudio.com/)

Recommended plugins:

* [Go Plugin](https://marketplace.visualstudio.com/items?itemName=golang.Go)

### JetBrains [GoLand](https://www.jetbrains.com/go/) or [IDEA](https://www.jetbrains.com/idea/)

Recommended plugins:

* [EnvFile Plugin](https://plugins.jetbrains.com/plugin/7861-envfile) to easily debug services running in kube.

## Platform Tips

### Linux

[Homebrew](https://brew.sh/) is great for getting newer development tools.  You can use it to install go, make, kubectl, etc.

### OS X / Darwin

[Homebrew](https://brew.sh/) is great for getting newer development tools.  You can use it to install go, make, kubectl, etc.

Latest version of OS X and Docker may not create the `/var/run/docker.sock` file.  If running e2e test fail due to this for you, run:

    sudo ln -s ~/.docker/run/docker.sock /var/run/docker.sock

### Windows

Install [WSL](https://learn.microsoft.com/en-us/windows/wsl/install) to get a linux shell.

Inside WSL [Homebrew](https://brew.sh/) is great for getting newer development tools.  You can use it to install go, make, kubectl, etc.

## Typical Development Workflow

Once you have checked out the source code with git run the service locally in kind using:

    make run-on-kind

If you make any changes to the service source code, run the following to get it redeployed in kind:

    make redeploy 

To validate you have not caused any regressions, run the e2e tests:

    make e2e

For a verbose output from e2e or build use `NOISY_BUILD`

     make e2e NOISY_BUILD=y

To run all the code generators, lint source code, and build the binaries for all platforms run:

    make all

For more fine grained make build targets, run make without any arguments:

    make
