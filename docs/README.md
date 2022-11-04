# Documentation

- [Documentation](#documentation)
  - [Concepts](#concepts)
  - [Deploying the Apex Controller](#deploying-the-apex-controller)
    - [Using docker-compose or podman-compose](#using-docker-compose-or-podman-compose)
    - [Run on Kubernetes](#run-on-kubernetes)
  - [The Apex Agent](#the-apex-agent)
    - [Installing the Agent](#installing-the-agent)
    - [Running the Agent for Interactive Enrollment](#running-the-agent-for-interactive-enrollment)
    - [Verifying Agent Setup](#verifying-agent-setup)
    - [Verifying Zone Connectivity](#verifying-zone-connectivity)
  - [Apex and connectivity scenarios:](#apex-and-connectivity-scenarios)
    - [1. Mesh Network between nodes deployed across different VPC and at Edge](#1-mesh-network-between-nodes-deployed-across-different-vpc-and-at-edge)
      - [**Setup the node that is getting onboarded to the mesh:**](#setup-the-node-that-is-getting-onboarded-to-the-mesh)
      - [**Start the Apex Controller stack**](#start-the-apex-controller-stack)
      - [**Generate private/public key pair for nodes**](#generate-privatepublic-key-pair-for-nodes)
      - [**Start the Apex agent**](#start-the-apex-agent)
    - [2. Multiple mesh network between nodes deployed across different VPC and at Edge (Multi-Tenancy)](#2-multiple-mesh-network-between-nodes-deployed-across-different-vpc-and-at-edge-multi-tenancy)
      - [**Create the zones via the REST API on the Apex Controller**](#create-the-zones-via-the-rest-api-on-the-apex-controller)
      - [**Verify the created zones**](#verify-the-created-zones)
      - [**Join the nodes to the zones**](#join-the-nodes-to-the-zones)
    - [3. Mesh network between containers running on connected nodes](#3-mesh-network-between-containers-running-on-connected-nodes)
    - [Additional Features supported by the project, not shown in the above examples:](#additional-features-supported-by-the-project-not-shown-in-the-above-examples)
    - [REST API](#rest-api)
    - [Cleanup](#cleanup)

## Concepts

- **Zone** - An isolated network connectivity domain. Apex supports multiple, isolated Zones.
- **Controller** - The Controller is the hosted service that handles authentication, authorization, management of zones, enrollment of nodes, and coordination among nodes to allow them to peer with other nodes.
- **Agent** - The Agent runs on any node which wants to join an Apex Zone.

## Deploying the Apex Controller

### Using docker-compose or podman-compose

For development and testing purposes, the quickest way to run the controller stack is by using `docker-compose` or `podman-compose`.

If you're using podman on Fedora (or a similar distribution), you may need to run this command before starting the stack. This resolve a volume permissions issue as we will be mounting this file as a volume into one of the containers.

```sh
chcon -R -t svirt_sandbox_file_t hack/controller-realm.json
```

In the rest of these commands, you may use `podman-compose` in place of `docker-compose` depending on your environment.

First, to build all required container images:

```sh
docker-compose build
```

To bring up the stack:

```sh
docker-compose up -d
```

To verify that everything has come up successfully:

```sh
$ curl http://localhost:8080/api/health
{"message":"ok"}
```

To tear everything back down:

```sh
docker-compose down
```

### Run on Kubernetes

Coming soon ...

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

## Apex and connectivity scenarios:

### 1. Mesh Network between nodes deployed across different VPC and at Edge

<img src="./images/caas-vpc-edge-single-zone.png" width="70%" height="70%" >

*Figure 1. Getting started topology that can be setup in minutes.* 

Please follow the instructions below to setup the connectivity scenario shown above.

#### **Setup the node that is getting onboarded to the mesh:**

You can directly build the required binaries from the source code

```shell
git clone https://github.com/redhat-et/apex
go install ./...
```
to build for Linux OS node
```shell
GOOS=linux GOARCH=amd64 go build -o apex-amd64-linux ./cmd/apex
```

to build for Mac OS node 
```shell
GOOS=darwin GOARCH=amd64 go build -o apex-amd64-darwin ./cmd/apex
```

Or download a recent binaries to the nodes:

*OSX Binary*

```shell
sudo curl https://apex-net.s3.amazonaws.com/apex-amd64-darwin --output /usr/local/sbin/apex
sudo chmod +x /usr/local/sbin/apex
```

*Linux Binary*
```shell
sudo curl https://apex-net.s3.amazonaws.com/apex-amd64-linux --output /usr/local/sbin/apex
sudo chmod +x /usr/local/sbin/apex
```

#### **Start the Apex Controller stack**
- Controller stack can run anywhere, as far as the apex agents (mentioned below) can reach it.
- The Controller must be running for agents to connect to the tunnel mesh. 
- If the Controller becomes unavailable, agent nodes continue functioning, only new nodes cannot join the mesh while it is down. 
- The same applies to the apex (agent), if the agent process exits, tunnels are maintained and only new peer joins are affected.

You can start the Controller stack using docker-compose or even podman-compose
```shell
docker-compose build
docker-compose up -d
```
You may opt not to use `docker-compose build` if you'd rather use prebuilt images from CI.
Ports are exposed to your host machine for ease of use.

#### **Generate private/public key pair for nodes**

- Keys are generated by the agent but a user can generate their own and play from in the following directory with the exact names and the agent will use that pair.

```shell
wg genkey | sudo tee /etc/wireguard/private.key | wg pubkey | sudo tee /etc/wireguard/public.key
```
For Windows and Mac adjust the paths to existing directories. 

**NOTE**: Make sure the node has wireguard installed. You can use following command to install wireguard on Ubuntu
 ```shell
 apt install wireguard-tools
 ```

#### **Start the Apex agent**
 Start the apex agent on the node you want to join the mesh network and fill in the relevant configuration. IP addressing of the mesh network is managed via the Apex Controller. Run the following commands on all the nodes:
 **Note**: If your test nodes are on private networks such as an EC2 VPC, you can use the `--stun` flag which will discovering your public address before NAT occurs. Alternatively, have total control over the endpoint IP and provide a specific address with `--local-endpoint-ip=x.x.x.x`:

There are currently 3 scenarios that allow an operator to define how the peers in a mesh are defined. There is a public address or cloud scenario, a private network address option and the ability to define exactly what address a peer will use when being mapped to a public key in the mesh. The following is an example of each:

1. If the node does not have inbound access on UDP port 51820 and a publicly reachable address,
   the following will use an existing IP on the node as the peer endpoint address. This would
   create internal peering to other nodes in your network or allow the node to initiate peers
   to public machines in the cloud. This is the default behavior as the vast majority of enterprise 
   hosts do not have public addresses with inbound traffic allowed.
```shell
sudo apex <CONTROLLER_URL>
```

2. If the node has access from the Internet allowed in on UDP port 51820 (AWS EC2 for example) the `--stun`
   flag will discover the node's public address and advertise to the mesh that discovered public NAT address as the endpoint address.
```shell
sudo apex <CONTROLLER_URL> \
    --stun
```

3. If an operator wants complete control over what address will be advertised to it's
   peers, they can specify the endpoint address that will be distributed to all of the other
   peers in the mesh (`--local-endpoint-ip`).

```shell
sudo apex <CONTROLLER_URL> \
    --local-endpoint-ip=X.X.X.X
```

**NOTES**
- *By default, the node joins a zone named `default`. A **zone** is simply the isolated wireguard network where all the nodes in that zone is connected as a mesh (as depicted in **Figure 1**).*
- *The default zone prefix is currently hardcoded to `10.200.1.0/20`. Custom zones and IPAM are in the next example.*
- *Keys are generated by default. A user can generate their own keys if they prefer. If a key pair exists on the host, those will be used instead of a new pair being generated. Private keys are never transmitted off the node.*
You will now have a flat host routed network between the endpoints. All of the wg0 (wireguard) interfaces can now reach one another. We currently work around NAT with a hacky STUN-like server to automatically discover public addressing for the user.


### 2. Multiple mesh network between nodes deployed across different VPC and at Edge (Multi-Tenancy)

This scenario shows how user can create multiple zone and connect the devices to these zone, to support multi tenant use cases. It also supports overlapping CIDR IPv4 or IPv6 across different zones.

<img src="./images/caas-vpc-dc-multi-zone.png" width="60%" height="60%" >

*Figure 2. Create multiple zones and connect nodes to each of these zone with network isolation.*

Follow the below instructions to setup this connectivity scenario. In this scenario, two zones are setup that are completely isolated from one another and can have overlapping CIDRs. If you were to add more nodes to either zone, the new nodes could communicate to other nodes in it's zone but not to a different zone. Zones are completely separate overlays and tenants.

**Note:** in the following example, the CIDR ranges can overlap since each zone is a separate peering mesh isolated from one another.

#### **Create the zones via the REST API on the Apex Controller**
In the following curl command, replace localhost with the IP address of the node the Controller is running on:

Create zone blue:
```shell
curl -L -X POST 'http://localhost:8080/api/zone' \
-H 'Content-Type: application/json' \
--data-raw '{
    "Name": "zone-blue",
    "Description": "Tenant - Zone Blue",
    "CIDR": "10.140.0.0/20"
}'
```

Create zone red:
```shell
curl -L -X POST 'http://localhost:8080/api/zone' \
-H 'Content-Type: application/json' \
--data-raw '{
    "Name": "zone-red",
    "Description": "Tenant - Zone Red",
    "CIDR": "172.20.0.0/20"
}'
```

#### **Verify the created zones**

```shell
curl -L -X GET 'http://localhost:8080/api/zones'
```

#### **Add users to selected zone**

```shell
curl -L -X PATCH -d '{ "zone_id": "$ZONE_ID" } "http://localhost:8080/api/users/${user-id}"
```

#### **Join the nodes to the zones**
Zone association is computed based on the User ID given to register the device

To join zone blue:
```shell
sudo apex <CONTROLLER_URL> --with-token=$ZONE_BLUE_USER_TOKEN
```
To join zone red:
```shell
sudo apex <CONTROLLER_URL> --with-token=$ZONE_RED_USER_TOKEN
```

Once you have more than one node in a zone, the nodes can now ping one another on using the wireguard interfaces. Get the address by running following command on the node:

```shell
# Linux
ip a wg0
# OSX - Note: OSX maps wg0 to tun(n). Generally 'ifconfig utun3' will show you the specific interface
ifconfig
```

You can also view the lease state of the IPAM objects with:

```shell
curl http://localhost:8080/api/ipam/leases/zone-blue
```
Curl should respond with output similar to the following

```
[
    {
        "Cidr": "10.140.0.0/20",
        "IPs": {
            "10.140.0.0": true,
            "10.140.0.1": true,
            "10.140.0.2": true,
            "10.140.15.255": true
        }
    }
]
```

**NOTES**
- If user would like to request a particular IP address from the IPAM module it can request the IP with the config option `--request-ip`. If an existing lease exists, it will be released and offered to the node requesting it. The IP you are requesting has to be in the CIDR range of the zone's prefix.



### 3. Mesh network between containers running on connected nodes

Imagine a user wants to not only communicate between the node address each member of the mesh but also want to advertise
some additional IP prefixes for additional services running on a node. This can be accomplished with the `--child-prefix` flag.
Prefixes have to be unique within a zone but can overlap on separate zones.

The following example allows a user to connect Docker container directly to one another without exposing a port on the node.
These nodes could be in different data centers or CSPs. This example uses the `--child-prefix` option to advertise the private
container networks to the mesh and enable connectivity as depicted below.

<img src="./images/caas-vpc-container-connectivity.png" width="60%" height="60%" >

*Figure 3. Encrypt and connect private RFC-1918 addresses and services in containers to all nodes in the mesh regardless of location*


For simplicity, we are just using the default, built-in zone `default`. You can also use the zones you created in the previous exercise or create a new one.

**Node1 setup:**

Join node1 to the `default` zone network
```shell
sudo apex <CONTROLLER_URL> \
    --child-prefix=172.24.0.0/24
```

Create the container network:
```shell
docker network create --driver=bridge --subnet=172.24.0.0/24 net1
```

Add the address range to the wg0 interface (required for docker only):
```shell
sudo iptables -I DOCKER-USER -i wg0 -d 172.24.0.0/24 -j ACCEPT
```

Start a container:
```shell
docker run -it --rm --network=net1 busybox bash
```

**Node2 setup**

Join node2 to the `default` zone network

```shell
sudo apex <CONTROLLER_URL> \
    --child-prefix=172.28.0.0/24
```

Setup a docker network and start a node on it:
```shell
docker network create --driver=bridge --subnet=172.28.0.0/24 net1
```

Add the address range to the wg0 interface (required for docker only):
```shell
sudo iptables -I DOCKER-USER -i wg0 -d 172.28.0.0/24 -j ACCEPT
```

Start a container:
```shell
docker run -it --rm --network=net1 busybox bash
```

ping the container started on Node1:
```shell
ping 172.28.0.x
```
If you don't want to create docker containers, you can create a loopback on each node's child prefix range and ping them from all nodes in the mesh like so:

*On Node1:*
```shell
sudo ip addr add 172.24.0.10/32 dev lo
```
*On Node1:*
```shell
sudo ip addr add 172.28.0.10/32 dev lo
```
Ping between nodes to the loopbacks, both IPs should be reachable now because those prefixes were added to the routing tables.

To go one step further, a user could then run apex on any machine, join the mesh and ping, or connect to a service, on both of the containers that were started. This could be a home developer's laptop, edge device, sensor or any other device with an IP address in the wild. That spoke connection does not require any ports to be opened to initiate the connection into the mesh.

```shell
sudo apex <CONTROLLER_URL>
```
Ping to prefixes on both the other nodes should be successful now.
```
ping 172.28.0.x
ping 172.24.0.x
```

**NOTES:**
- once you allocate a prefix, it is fixed in IPAM. We do not currently support removing the prefix. If you want to
add different child prefix, either use a different cidr or delete the persistent state file in the root of where you ran the 
Controller binary named `<zone-name>.json`. For example, `ipam-red.json`.

- Containers need to have unique private addresses on the docker network as exemplified above. Overlapping addresses 
within a zone is not supported because that is a nightmare to troubleshoot, creates major fragility in SDN deployments and is 
all around insanity. TLDR; IP address management in v4 networks is important when deploying infrastructure ¯\_(ツ)_/¯ 


### Additional Features supported by the project, not shown in the above examples:

- You can also run the apex command on one node and then run the exact same command and keys on a new node and the assigned address from the Apex Controller will move that peering
  from to the new machine you run it on along with updating the mesh as to the new endpoint address.
- This can be run behind natted networks for remote spoke machines and do not require any incoming ports to be opened to the device. Only one side of the peering needs an open port
  for connections to be initiated. Once the connection is initiated from one side, bi-directional communications can be established. This aspect is especially interesting for IOT/Edge.
- An IPAM module handles node address allocations but also allows the user to specify it's wireguard node address.


### REST API

There are currently some supported REST calls:

**Get all peers:**

```shell
curl -s --location --request GET 'http://localhost:8080/api/peers' | python -m json.tool
```

*Output:*

```json
[{
"PublicKey": "DUQ+TxqMya3YgRd1eXW/Tcg2+6wIX5uwEKqv6lOScAs=",
"EndpointIP": "3.94.59.204:51820",
"AllowedIPs": "10.20.1.1/32",
"Zone": "zone-blue"
},
{
"PublicKey": "O3UVnLl6BFNYWf21tEDGpKbxYfzCp9LzwSXbtd9i+Eg=",
"EndpointIP": "18.205.149.74:51820",
"AllowedIPs": "10.20.1.2/32",
"Zone": "zone-blue"
},
{
"PublicKey": "SvAAJctGA5U6EP+30LMuhoG76VLrEhwq3rwFf9pqcB4=",
"EndpointIP": "3.82.51.92:51820",
"AllowedIPs": "10.20.1.3/32",
"Zone": "zone-blue"
},
{
"PublicKey": "M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=",
"EndpointIP": "34.224.78.66:51820",
"AllowedIPs": "10.20.1.1/32",
"Zone": "zone-red"
},
{
"PublicKey": "oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=",
"EndpointIP": "71.31.21.22:51820",
"AllowedIPs": "10.20.1.2/32",
"Zone": "zone-red"
},
{
"PublicKey": "IMqxPz/eQzCdHjb8Ajl7OVTtJmZqiKeS6SvQLml21nU=",
"EndpointIP": "71.31.21.22:51820",
"AllowedIPs": "10.20.1.3/32",
"Zone": "zone-red"
}]
```

**Get a peer by key:**

```shell
curl -s --location --request GET 'http://localhost:8080/api/peers/M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=' | python -m json.tool
```

*Output:*

```json
{
    "PublicKey": "M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=",
    "EndpointIP": "34.224.78.66:51820",
    "AllowedIPs": "10.20.1.1/32",
    "Zone": "zone-red"
}
```

**Get zone details:**

```shell
curl --location --request GET 'http://localhost:8080/api/zones'
```

*Output:* **(notice the overlapping CIDR address support)**

```json
[
  {
    "Name": "zone-red",
    "Description": "Tenancy Zone Red",
    "IpCidr": "10.20.1.0/20"
  },
  {
    "Name": "zone-blue",
    "Description": "Tenancy Zone Blue",
    "IpCidr": "10.20.1.0/20"
  }
]
```

**Get the leases of nodes in a particular zone:**

```shell
curl --location --request GET 'http://localhost:8080/api/ipam/leases/zone-blue'
curl --location --request GET 'http://localhost:8080/api/ipam/leases/zone-red'
```

*Output:*

```json
[
    {
        "Cidr": "10.20.0.0/20",
        "IPs": {
            "10.20.0.0": true,
            "10.20.0.1": true,
            "10.20.0.2": true,
            "10.20.0.29": true,
            "10.20.0.3": true,
            "10.20.0.4": true,
            "10.20.0.5": true,
            "10.20.0.6": true,
            "10.20.15.255": true
        }
    }
]
```

### Cleanup
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
