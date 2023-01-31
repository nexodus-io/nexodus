# Documentation

- [Documentation](#documentation)
  - [Concepts](#concepts)
  - [Deploying the Apex Controller](#deploying-the-apex-controller)
    - [Run on Kubernetes](#run-on-kubernetes)
      - [Add required DNS entries](#add-required-dns-entries)
      - [Deploy using KIND](#deploy-using-kind)
    - [HTTPS](#https)
  - [Deploying the Apexctl Utility](#deploying-the-apexctl-utility)
    - [Install pre-built binary](#install-pre-built-binary)
    - [Build from the source code](#build-from-the-source-code)
  - [The Apex Agent](#the-apex-agent)
    - [Installing the Agent](#installing-the-agent)
    - [Running the Agent for Interactive Enrollment](#running-the-agent-for-interactive-enrollment)
    - [Verifying Agent Setup](#verifying-agent-setup)
    - [Verifying Zone Connectivity](#verifying-zone-connectivity)
    - [Cleanup](#cleanup)
  - [Additional Features](#additional-features)
    - [Subnet Routers](#subnet-routers)
  - [Running the integration tests](#running-the-integration-tests)
    - [Prerequisites](#prerequisites)
    - [Using Docker](#using-docker)
    - [Using podman](#using-podman)

## Concepts

- **Zone** - An isolated network connectivity domain. Apex supports multiple, isolated Zones.
- **Controller** - The Controller is the hosted service that handles authentication, authorization, management of zones, enrollment of nodes, and coordination among nodes to allow them to peer with other nodes.
- **Agent** - The Agent runs on any node which wants to join an Apex Zone.

## Deploying the Apex Controller

### Run on Kubernetes

#### Add required DNS entries

The development Apex stack requires 3 hostnames to be reachable:

- `auth.apex.local` - for the authentication service
- `api.apex.local` - for the backend apis
- `apex.local` - for the frontend

To add these on your own machine for a local development environment:

```console
echo "127.0.0.1 auth.apex.local api.apex.local apex.local" | sudo tee -a /etc/hosts
```

#### Deploy using KIND

> **Note**
> This section is only if you want to build the controller stack. If you want to attach to a running controller, see [The Apex Agent](#the-apex-agent).

You should first ensure that you have `kind`, `kubectl` and [`mkcert`](https://github.com/FiloSottile/mkcert) installed.

If not, you can follow the instructions in the [KIND Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/).

```console
make run-on-kind
```

This will install:

- `apex-dev` kind cluster
- `ingress-nginx` ingress controller
- a rewrite rule in coredns to allow `auth.apex.local` to resolve inside the k8s cluster
- the `apex` stack

To bring the cluster down again:

```console
make teardown
```

### HTTPS

The Makefile will install the https certs. You can view the cert in the Apex root where you ran the Makefiile.

```console
cat .certs/rootCA.pem
```

In order to join a self-signed Apex controller from a remote node or view the Apex UI in your dev environment, you will need to install the cert on the remote machine. This is only necessary when the controller is self-signed with a domain like we are using with the apex.local domain for development.

Install [`mkcert`](https://github.com/FiloSottile/mkcert) on the agent node, copy the cert from the controller running kind (`.certs/rootCA.pem`) to the remote node you will be joining (or viewing the web UI) and run the following. 

```console
CAROOT=$(pwd)/.certs mkcert -install
```

For windows, we recommend installing the root certificate via the [MMC snap-in](https://learn.microsoft.com/en-us/troubleshoot/windows-server/windows-security/install-imported-certificates#import-the-certificate-into-the-local-computer-store).

## Using the Apexctl Utility

`apexctl` is a CLI utility that is used to interact with the Apex Api Server. It provides command line options to get existing configuration of the resources like Zone, Peer, User and Devices from the Api server. It also allows limited options to configure certain aspects of these resources. Please use `apexctl -h` to learn more about the available options.

You can install `apexctl` using following two ways

### Install pre-built binary

You can directly fetch the binary from the Apex's AWS S3 bucket.

```sh
sudo curl -fsSL https://apex-net.s3.amazonaws.com/apexctl-linux-amd64 --output /usr/local/sbin/apexctl
sudo chmod a+x /usr/local/sbin/apexctl
```

### Build from the source code

You can clone the Apex repo and build the binary using

```sh
make dist/apexctl
```

## The Apex Agent

### Installing the Agent

The Apex agent (`apexd`) is run on any node that will join an Apex Zone to communicate with other peers in that zone. This agent communicates with the Apex Controller and manages local wireguard configuration.

The `hack/apex_installer.sh` script will download the latest build of `apexd` and install it for you. It will also ensure that `wireguard-tools` has been installed. This installer supports MacOS and Linux. You may also install `wireguard-tools` yourself and build `apexd` from source.

```sh
hack/apex_installer.sh
```

### Running the Agent for Interactive Enrollment

As the project is still in such early development, it is expected that `apexd` is run manually on each node you intend to test. If the agent is able to successfully reach the controller API, it will provide a one-time code to provide to the controller web UI to complete enrollment of this node into an Apex Zone.

Note: In a self-signed dev environment, each agent machine needs to have the [imported cert](#https) and the [host entry](#add-required-dns-entries) detailed above.

```sh
sudo apexd-linux-amd64 https://apex.local
Your device must be registered with Apex.
Your one-time code is: LTCV-OFFS
Please open the following URL in your browser to sign in:
https://auth.apex.local/realms/apex/device?user_code=LTCV-OFFS
```

Once enrollment is completed in the web UI, the agent will show progress.

```text
Authentication succeeded.
...
INFO[0570] Peer setup complete
```

### Verifying Agent Setup

Once the Agent has been started successfully, you should see a wireguard interface with an address assigned. For example, on Linux:

```sh
$ ip address show wg0
161: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 10.200.0.1/32 scope global wg0
       valid_lft forever preferred_lft forever
```

### Verifying Zone Connectivity

Once more than one node has enrolled in the same Apex Zone, you will see additional routes populated for reaching other node's endpoints in the same Zone. For example, we have just added a second node to this zone. The new node's address in the Apex Zone is 10.200.0.2. On Linux, we can check the routing table and see:

```sh
$ ip route
...
10.200.0.2 dev wg0 scope link
```

You should now be able to reach that node over the wireguard tunnel.

```sh
$ ping 10.200.0.2
PING 10.200.0.2 (10.200.0.2) 56(84) bytes of data.
64 bytes from 10.200.0.2: icmp_seq=1 ttl=64 time=7.63 ms
```

You can explore the web UI by visiting the URL of the host you added in your `/etc/hosts` file. For example, `https://apex.local/`.

### Cleanup

If you want to remove the node from the network, and want to cleanup all the configuration done on the node. Fire away following commands:

Ctrl + c (cmd+c) the agent process. and remove the wireguard interface and relevant configuration files.
*Linux:*

```shell
sudo ip link del wg0
```

*OSX/Windows:*

Since the wireguard agents are userspace in both Windows and Darwin, the tunnel interface is removed when the agent process exits.

## Additional Features

### Subnet Routers

Typically, the Apex agent runs on every host that you intend to have connectivity to an Apex Zone. However, there may be some cases where you can't do that or don't want to. It is also possible to make a host act as a Subnet Router to provide connectivity between an Apex Zone and a local Subnet the host has access to.

In the following diagram, `Host X` acts as a Subnet Router, allowing all hosts within Apex Zone A to access `192.168.100.0/24`.

To configure this scenario, the `apexd` agent on `Host X` must be run with the `--child-prefix` parameter.

```sh
sudo apexd --child-prefix 192.168.100.0/24 [...]
```

The subnet exposed to the Apex Zone may be a physical network the host is connected to, but it can also be a network local to the host. This works well for exposing a local subnet used for containers running on that host. A demo of this containers use case can be found in [scenarios/containers-on-nodes.md](scenarios/containers-on-nodes.md).

> **Note**
> Subnet Routers do not perform NAT. Routes for hosts in `192.168.100.0/24` to reach Apex Zone A via `Host X` must be handled via local configuration that is appropriate for your network.

```mermaid
graph
    subgraph "Apex Zone - 10.0.0.10/24"
        x[Host X]<---> y
        y[Host Y]<---> z[Host Z]
        x<--->z
    end

    x <---> s[Subnet Accessible by Host X<br/>192.168.100.0/24]
```

## Running the integration tests

### Prerequisites

Prior to running the integration tests, you must start the kind-based development environment. See the [Run on Kubernetes](#run-on-kubernetes) section for further details.

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
sudo APEX_TEST_PODMAN=1 TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED=true ./integration-tests.test -test.v
```
