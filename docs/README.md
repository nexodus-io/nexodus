# Documentation

- [Documentation](#documentation)
  - [Concepts](#concepts)
  - [Deploying the Apex Controller](#deploying-the-apex-controller)
    - [Run on Kubernetes](#run-on-kubernetes)
      - [Add required DNS entries](#add-required-dns-entries)
      - [Deploy using KIND](#deploy-using-kind)
    - [HTTPS](#https)
  - [Using the Apexctl Utility](#using-the-apexctl-utility)
    - [Install pre-built binary](#install-pre-built-binary)
    - [Build from the source code](#build-from-the-source-code)
  - [Deploying the Apex Agent](#deploying-the-apex-agent)
    - [Deploying on Node](#deploying-on-node)
      - [Installing the Agent](#installing-the-agent)
        - [Install Script](#install-script)
        - [RPM](#rpm)
        - [Systemd](#systemd)
        - [Starting the Agent](#starting-the-agent)
      - [Interactive Enrollment](#interactive-enrollment)
      - [Verifying Agent Setup](#verifying-agent-setup)
      - [Verifying Zone Connectivity](#verifying-zone-connectivity)
      - [Cleanup Agent From Node](#cleanup-agent-from-node)
    - [Deploying on Kubernetes managed Node](#deploying-on-kubernetes-managed-node)
      - [Setup the configuration](#setup-the-configuration)
      - [Deploying the Agent in the Kind Dev Environment](#deploying-the-agent-in-the-kind-dev-environment)
      - [Deploying the Apex Agent Manifest](#deploying-the-apex-agent-manifest)
      - [Controlling the Agent Deployment](#controlling-the-agent-deployment)
      - [Verify the deployment](#verify-the-deployment)
      - [Cleanup Agent Pod From Node](#cleanup-agent-pod-from-node)
  - [Deploying the Apex Relay](#deploying-the-apex-relay)
    - [Setup Apex Relay Node](#setup-apex-relay-node)
    - [Create a Relay Enabled Zone](#create-a-relay-enabled-zone)
    - [Move user to Relay Enabled Zone](#move-user-to-relay-enabled-zone)
    - [OnBoard the Relay node to the Relay Enabled Zone](#onboard-the-relay-node-to-the-relay-enabled-zone)
      - [Interactive OnBoarding](#interactive-onboarding)
      - [Silent OnBoarding](#silent-onboarding)
    - [Delete Zone](#delete-zone)
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
> This section is only if you want to build the controller stack. If you want to attach to a running controller, see [Deploying the Apex Agent](#deploying-the-apex-agent).

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

## Deploying the Apex Agent

### Deploying on Node

The following sections contain general instructions to deploy the Apex agent on any node. The minimum requirement is that the node runs Linux, Darwin, or Windows-based Operating systems.

#### Installing the Agent

##### Install Script

The Apex agent (`apexd`) is run on any node that will join an Apex Zone to communicate with other peers in that zone. This agent communicates with the Apex Controller and manages local wireguard configuration.

The `hack/apex_installer.sh` script will download the latest build of `apexd` and install it for you. It will also ensure that `wireguard-tools` has been installed. This installer supports MacOS and Linux. You may also install `wireguard-tools` yourself and build `apexd` from source.

```sh
hack/apex_installer.sh
```

##### RPM

You can build an rpm from the git repository. The rpm will include `apexctl`, `apexd`, and integration with systemd. You must have `mock` installed to build the package.

```sh
make rpm
```

After running this command, the resulting rpm can be found in `./dist/rpm/mock/`.

To install the rpm, you may use `dnf`.

```sh
sudo dnf install ./dist/rpm/mock/apex-0-0.1.20230216git068fedd.fc37.src.rpm
```

##### Systemd

If you did not install `apexd` via the rpm, you can still use the systemd integration if you would like. The following commands will put the files in the right place.

```sh
sudo cp contrib/rpm/apex.service /usr/lib/systemd/service/apex.service
sudo cp contrib/rpm/apex.sysconfig /etc/sysconfig/apex
sudo systemctl daemon-reload
```

##### Starting the Agent

> **Note**
> In a self-signed dev environment, each agent machine needs to have the [imported cert](#https) and the [host entry](#add-required-dns-entries) detailed above.

You may start `apexd` directly. You must include the URL to the Apex service as an argument.

```sh
sudo apexd-linux-amd64 https://apex.local
```

Alternatively, you can start `apexd` as a systemd service. First, edit `/etc/sysconfig/apex` to reflect the URL of the Apex service. Then, start the agent with the following command:

```sh
sudo systemctl start apex
```

If you would like `apexd` to run automatically on boot, run this command as well:

```sh
sudo systemctl enable apex
```

#### Interactive Enrollment

If the agent is able to successfully reach the controller API, it will provide a one-time code to provide to the controller web UI to complete enrollment of this node into an Apex Zone. If you ran `apexd` manually, you will see a message like the following in your terminal:

```sh
Your device must be registered with Apex.
Your one-time code is: LTCV-OFFS
Please open the following URL in your browser to sign in:
https://auth.apex.local/realms/apex/device?user_code=LTCV-OFFS
```

If the agent was started using systemd, you will find the same thing in the service logs.

Once enrollment is completed in the web UI, the agent will show progress.

```text
Authentication succeeded.
...
INFO[0570] Peer setup complete
```

#### Verifying Agent Setup

Once the Agent has been started successfully, you should see a wireguard interface with an address assigned. For example, on Linux:

```sh
$ ip address show wg0
161: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 10.200.0.1/32 scope global wg0
       valid_lft forever preferred_lft forever
```

#### Verifying Zone Connectivity

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

#### Cleanup Agent From Node

If you want to remove the node from the network, and want to cleanup all the configuration done on the node. Fire away following commands:

Ctrl + c (cmd+c) the agent process. and remove the wireguard interface and relevant configuration files.
*Linux:*

```shell
sudo ip link del wg0
```

*OSX/Windows:*

Since the wireguard agents are userspace in both Windows and Darwin, the tunnel interface is removed when the agent process exits.

### Deploying on Kubernetes managed Node

Instructions mentioned in [Deploying on Node](#deploying-on-node) can be used here to deploy the Apex agent on Kubernetes-managed nodes. However, deploying the agent across all the nodes in a sizable Kubernetes cluster can be a challenging task. The following section provides instructions to deploy Kubernetes style manifest to automate the deployment process.

#### Setup the configuration

Agent deployment in Kubernetes requires a few initial configuration details to successfully deploy the agent and onboard the node. This configuration is provided through `kustomization.yaml`. Make a copy of the sample `kustomization.yaml.sample`](../deploy/apex-client/overlays/dev/kustomization.yaml.sample) and rename it to `kustomization.yaml`.

Fetch the CA cert from the Kubernetes cluster that is running the Apex controller, and set the return cert string to `cert` literal of `secretGenerator` in `kustomization.yaml`.

```sh
kubectl get secret -n apex apex-ca-key-pair -o json | jq -r '.data."ca.crt"'
```

Update the `<APEX_CONTROLLER_IP>` with the IP address where the Apex controller is reachable. If you are running the Apex stack in Kind cluster on your local machine, set it to the public IP of the machine.

**Note**
Current username and password are default configuration for development environment, which is very likely to change in near future.

You can refer to [kustomization.yaml.sample](../deploy/apex-client/overlays/dev/kustomization.yaml.sample) for an example manifest.

If you have setup your Apex stack with non-default configuration, please copy the [sample](./../deploy/apex-client/overlays/sample/) directory and update the sample file according to create a new overlay for your setup and deploy it.

#### Deploying the Agent in the Kind Dev Environment

If you're using the kind-based development environment, you can deploy the agent to each node in that cluster using this command:

```sh
make deploy-apex-agent
```

Then you may skip down to the [Verify the deployment](#verify-the-deployment) section.

Otherwise, if you are working with another cluster, continue to the next section.

#### Deploying the Apex Agent Manifest

Once the configuration is set up, you can deploy Apex's manifest files.

```sh
kubectl apply -k ./deploy/apex-client/overlays/dev
```

It will deploy a DaemonSet in the newly created `Apex` namespace. DaemonSet deploys a Pod that runs a privileged container that does all the required configuration on the local node. It also starts the agent and onboard the device automatically using the non-interactive access token based onboarding.

**Note**
If your Kubernetes cluster enforces security context to deny privileged container deployment, you need to make sure that the security policy is added to the service account `apex` (created for agent agent deployment) to allow the deployment.

#### Controlling the Agent Deployment

By default Apex agent is deployed using DaemonSet, so Kubernetes will deploy Apex agent pod on each worker node. This might not be the ideal strategy for onboarding Kubernetes worker nodes for many reasons. You can control the Apex agent deployment by configuring the `nodeAffinity` in the [node_selector.yaml](../deploy/apex-client/overlays/dev/node_selector.yaml).

The default behavior is set to deploy Apex pod on any node that is running Linux Operating System and is tagged with `app.kubernetes.io/apex=`. With this deployment strategy, once you apply the Apex manifest, Kubernetes won't deploy Apex pod on any worker node. To deploy the Apex pod on any worker node, tag that node with `app.kubernetes.io/apex=` label.

```sh
kubectl label nodes <NODE_NAME> app.kubernetes.io/apex=
```

If you want to remove the deployment from that node, just remove the label.

```sh
kubectl label nodes <NODE_NAME> app.kubernetes.io/apex-
```

If you want to change the deployment strategy for Apex pod, please copy the [sample](./../deploy/apex-client/overlays/sample/) directory to create a new overlay, and configure the  [node_selector.yaml.sample](../deploy/apex-client/overlays/sameple/node_selector.yaml.sample) file as per your requirements. After making the required changes rename the file to `node_selector.yaml` and deploy it.

Currently sample file provides two strategy to control the deployment, but feel free to change it based on your requirements.

1 ) Deploy Apex pod on any node that is tagged with `app.kubernetes.io/apex=`

```yaml
  - key: app.kubernetes.io/apex
    operator: Exists
```

2 ) Deploy Apex pod on specific node/s in the Kubernetes cluster. Uncomment the following lines in `node_selector.yaml.sample` and add the list of the nodes.

```yaml
# Deploy apex client on  specific nodes
#              - key: kubernetes.io/hostname
#                operator: In
#                values:
#                - <NODE_1>
#                - <NODE_2>
```

#### Verify the deployment

Check that apex pod is running on the nodes

```sh
kubectl get pods -n apex -o wide
```

Login to one of the nodes, and you should see a wireguard interface with an address assigned. For example, on Linux:

```sh
$ ip address show wg0
161: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 10.200.0.1/32 scope global wg0
       valid_lft forever preferred_lft forever
```

#### Cleanup Agent Pod From Node

Removing the Apex manifest will remove the Apex pod and clean up WireGuard configuration from the node as well.

```sh
kubectl delete -k ./deploy/apex-client/overlays/dev
```

## Deploying the Apex Relay

Apex Controller makes best effort to establish a direct peering between the endpoints, but in some scenarios such as symmetric NAT, it's not possible to establish the direct peering. To establish connectivity in those scenario, Apex Controller uses Apex Relay to relay the traffic between the endpoints. To use this feature you need to onboard a Relay node to the Apex network. This **must** be the first device to join the Apex network to enable the traffic relay.

### Setup Apex Relay Node

Clone the Apex repository on a VM (or bare metal machine). Apex relay node must be reachable from all the endpoint nodes that want to join the Apex network. Follow the instruction in [Installing the agent](#installing-the-agent) section to setup the node and install the apex binary.

### Create a Relay Enabled Zone

Use the following cli command to create the Relay enabled Apex Zone. You can login to the Apex UI and create the zone as well, ensuring that you toggle the `Hub Zone` switch to on.

```sh
./apexctl --username=kitteh1 \
   --password=floofykittens \
   zone create --name=edge-zone \
   --hub-zone=true \
   --cidr=10.195.0.0/24 \
   --description="Relay enabled zone"
```

Currently for the Dev environment, the usernames and passwords are hardcoded in the Apex Controller.
You can edit `name`, `cidr`, `description` parameters in the CLI commands as per your deployment.

You can list the available zones using following command

```sh
./apexctl  --username=kitteh1 --password=floofykittens zone list                                                         

ZONE ID                                  NAME          CIDR              DESCRIPTION                RELAY/HUB ENABLED
dcab6a84-f522-4e9b-a221-8752d505fc18     default       10.200.1.0/20     Default Zone               false
b130805a-c312-4f4a-8a8e-f57a2c7ab152     edge-zone     10.195.0.0/24     Relay enabled zone         true
```

You can see the two zones associated with the username `kitteh1`.

### Move user to Relay Enabled Zone

By default, a user account is associated with its default zone, that is not a relay enabled zone. So to onboard all the devices to the relay enabled zone, you need to associate that zone as a default zone of the user account.

```sh
apexctl  --username=kitteh1 --password=floofykittens zone move-user --zone-id b130805a-c312-4f4a-8a8e-f57a2c7ab152
```

Zone id is printed once you create the zone, or you can get it by listing the zone as mentioned above.

### OnBoard the Relay node to the Relay Enabled Zone

Once the user account is associated with the newly created zone, any OnBoarded device will join the new zone. To OnBoard the Relay node you can use any of the following method

#### Interactive OnBoarding

```sh
sudo apex --hub-router --stun https://apex.local
```

It will print an URL on stdout to onboard the relay node

```sh
$ sudo apex --hub-router --stun https://apex.local
Your device must be registered with Apex.
Your one-time code is: GTLN-RGKP
Please open the following URL in your browser to sign in:
https://auth.apex.local/device?user_code=GTLN-RGKP
```

Open the URL in your browser and provide the username and password that you used to create the zone, and follow the GUI's instructions. Once you are done granting the access to the device in the GUI, the relay node will be OnBoarded to the Relay Zone.

#### Silent OnBoarding

To OnBoard devices without any browser involvement you need to provide username and password in the CLI command

```sh
apex --hub-router --stun --username=kitteh1 --password=floofykittens https://apex.local
```

### Delete Zone

To delete the zone,

```sh
apexctl  --username=kitteh1 --password=floofykittens zone delete --zone-id b130805a-c312-4f4a-8a8e-f57a2c7ab152
```

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
