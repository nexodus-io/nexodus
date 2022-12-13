# Documentation

- [Documentation](#documentation)
  - [Concepts](#concepts)
  - [Deploying the Apex Controller](#deploying-the-apex-controller)
    - [Run on Kubernetes](#run-on-kubernetes)
      - [Add required DNS entries](#add-required-dns-entries)
      - [Deploy using KIND](#deploy-using-kind)
    - [HTTPS](#https)
  - [The Apex Agent](#the-apex-agent)
    - [Installing the Agent](#installing-the-agent)
    - [Running the Agent for Interactive Enrollment](#running-the-agent-for-interactive-enrollment)
    - [Verifying Agent Setup](#verifying-agent-setup)
    - [Verifying Zone Connectivity](#verifying-zone-connectivity)
  - [Cleanup](#cleanup)
  - [Running the integration tests](#running-the-integration-tests)
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

To add these on your own machine:

```console
echo "127.0.0.1 auth.apex.local api.apex.local apex.local" | sudo tee -a /etc/hosts
```

#### Deploy using KIND

You should first ensure that you have `kind` and `kubectl` installed.
If not, you can follow the instructions in the [KIND Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/).

```console
./hack/kind/kind.sh up
```

This will install:

- `apex-dev` kind cluster
- `ingress-nginx` ingress controller
- a rewrite rule in coredns to allow `auth.apex.local` to resolve inside the k8s cluster
- the `apex` stack

To bring the cluster down again:

```console
./hack/kind/kind.sh down
```

### HTTPS

You will need to extract the CA certificate and add it to your system trust store:

```console
mkdir -p .certs
kubectl get secret -n apex apex-ca-key-pair -o json | jq -r '.data."ca.crt"' | base64 -d > .certs/rootCA.pem
```

Installation of this certificate is best left to [`mkcert`](https://github.com/FiloSottile/mkcert)

```console
CAROOT=$(pwd)/.certs mkcert -install
```

## The Apex Agent

### Installing the Agent

The Apex agent is run on any node that will join an Apex Zone to communicate with other peers in that zone. This agent communicates with the Apex Controller and manages local wireguard configuration.

The `hack/apex_installer.sh` script will download the latest build of `apex` and install it for you. It will also ensure that `wireguard-tools` has been installed. This installer supports MacOS and Linux. You may also install `wireguard-tools` yourself and build `apex` from source.

```sh
hack/apex_installer.sh
```

### Running the Agent for Interactive Enrollment

As the project is still in such early development, it is expected that `apex` is run manually on each node you intend to test. If the agent is able to successfully reach the controller API, it will provide a one-time code to provide to the controller web UI to complete enrollment of this node into an Apex Zone.

```sh
$ sudo ./apex <CONTROLLER_API_IP>:<CONTROLLER_API_PORT>
Your device must be registered with Apex Controller.
Your one-time code is: ????-????
Please open the following URL in your browser and enter your one-time code:
http://HOST:PORT/auth/realms/controller/device
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
$ ping -c 1 10.200.0.2
PING 10.200.0.2 (10.200.0.2) 56(84) bytes of data.
64 bytes from 10.200.0.2: icmp_seq=1 ttl=64 time=7.63 ms
```

## Cleanup

If you want to remove the node from the network, and want to cleanup all the configuration done on the node. Fire away following commands:

Ctrl + c (cmd+c) the agent process. and remove the wireguard interface and relevant configuration files.
*Linux:*

```shell
sudo rm /etc/wireguard/wg0-latest-rev.conf
sudo rm /etc/wireguard/wg0.conf
sudo ip link del wg0
```

*Mac-OSX:*

```shell
sudo wg-quick down wg0
```

## Running the integration tests

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
