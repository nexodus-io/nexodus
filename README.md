# Jaywalking

[![build](https://github.com/redhat-et/jaywalking/actions/workflows/build.yml/badge.svg)](https://github.com/redhat-et/jaywalking/actions/workflows/build.yml)

Roads? Where we're going, we don't need roads - *Dr Emmett Brown*

### Jaywalk Quickstart

<img src="https://jaywalking.s3.amazonaws.com/jaywalker-multi-tenant.png" width="100%" height="100%">

*Figure 1. Getting started topology that can be setup in minutes.* 

- Build for the node OS that is getting onboarded to the mesh:

```shell
git clone https://github.com/redhat-et/jaywalking
cd jaywalking
cd jaywalk-agent
GOOS=linux GOARCH=amd64 go build -o jaywalk-amd64-linux
GOOS=darwin GOARCH=amd64 go build -o jaywalk-amd64-darwin
```


- Or download a recent build to the nodes you are looking to add to the mesh:

__OSX Binary__

```shell
sudo curl https://jaywalking.s3.amazonaws.com/jaywalk-amd64-darwin --output /usr/local/sbin/jaywalk
sudo chmod +x /usr/local/sbin/jaywalk
```

__Linux Binary__
```shell
sudo curl https://jaywalking.s3.amazonaws.com/jaywalk-amd64-linux --output /usr/local/sbin/jaywalk
sudo chmod +x /usr/local/sbin/jaywalk
```

- Start a redis instance in EC2 or somewhere all nodes can reach (below is an example for podman or docker for ease of use, no other configuration is required):

```shell
docker run \
    --name redis \
    -d -p 6379:6379 \
    redis redis-server \
    --requirepass <REDIS_PASSWD>
```

- Verify that container is up and redis server is in running state 
```shell
docker run -it --rm redis redis-cli -h <container-host-ip> -a <REDIS_PASSWD> --no-auth-warning PING
```
If it outputs **PONG**, that's a success.


- Start the supervisor/controller SaaS portion (this can be your laptop, the only requirement is it can reach the redis streamer started above). 
- The supervisor must be running for agents to connect to the tunnel mesh. 
- If the supervisor becomes unavailable, agent nodes continue functioning, only new nodes cannot join the mesh while it is down. The same applies 
to the agent, if the agent process exits, tunnels are maintained and only new peer joins are affected.

```shell
git clone https://github.com/redhat-et/jaywalking.git

cd supervisor
go build -o jaywalk-supervisor

./jaywalk-supervisor \
    -streamer-address <REDIS_SERVER_ADDRESS> \
    -streamer-passwd <REDIS_PASSWD>
```

- Generate private/public key pair for the nodes that you want to connect in the mesh network. For a Linux node run the following. For Windows and Mac adjust the paths to existing directories. ex. ~/.wireguard/

```shell
wg genkey | sudo tee /etc/wireguard/server_private.key | wg pubkey | sudo tee /etc/wireguard/server_public.key
```
 NOTE: Make sure the node has wireguard installed. You can use following command to install wireguard on Ubuntu
 ```shell
 apt install wireguard-tools
 ```

- Start the jaywalk agent on the node you want to join the mesh and fill in the relevant configuration. IP addressing of the mesh network is managed via the controller. Run the following on a few nodes and set up a mesh.
- *Note:* while we pass the private key via CLI in the examples (dev/demo purposes only), we would highly recommend using the cli flag `--private-key-file=/path/to/private.key` or ENV `JAYWALK_PRIVATE_KEY_FILE=/path/to/private.key` in all scenarios where key safety protection is an issue.

```shell
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
     --agent-mode
```
- By default, the network joins a zone named `default`. A zone is simply the isolated wireguard network mesh as depicted in **Figure 1**.
- - The default zone prefix is currently hardcoded to `10.200.1.0/20`. Custom zones and IPAM are in the next example.
- You will now have a flat host routed network between the endpoints. All of the wg0 interfaces can now reach one another. We currently work around NAT with a hacky STUN-like server to automatically discover public addressing for the user.

- Cleanup

```shell
# Quick Linux
sudo ip link del wg0

# Complete Linux
sudo rm /etc/wireguard/wg0-latest-rev.conf
sudo rm /etc/wireguard/wg0.conf
sudo ip link del wg0

# OSX - simply ctrl^c the agent or the following
sudo wg-quick down wg0
```

### Additional Features

- This also provides multi-tenancy and overlapping CIDR IPv4 or IPv6 by creating new zones and populating them via the agent switch `--zone=X` or `--zone=Y`.
- You can also run the jaywalk command on one node and then run the exact same command and keys on a new node and the assigned address from the supervisor will move that peering
  from to the new machine you run it on along with updating the mesh as to the new endpoint address.
- This can be run behind natted networks for remote spoke machines and do not require any incoming ports to be opened to the device. Only one side of the peering needs an open port
  for connections to be initiated. Once the connection is initiated from one side, bi-directional communications can be established. This aspect is especially interesting for IOT/Edge.
- An IPAM module handles node address allocations but also allows the user to specify it's wireguard node address.

### Multi-Tenancy 

- Another join example from a node includes the ability to specify what zone to join and allows you to request a particular IP address from the IPAM module with the request ip switch `--request-ip`. If an existing lease exists, it
will be released and offered to the node requesting it. The IP you are requesting has to be in the CIDR range of the zone's prefix.
- In the following example, two zones are setup that are completely isolated from one another and can have overlapping CIDRs. If you were to add more nodes to either zone, the new nodes could 
communicate to other nodes in it's zone but not to a different zone. Zones are completely separate overlays and tenants.


- **Note:** in the following example, the CIDR ranges can overlap since each zone is a separate peering mesh isolated from one another.
- First, create the zones via the REST API on the supervisor with the following (replace localhost with the IP address of the node the supervisor is running on):

```shell
# Create Zone - zone-blue
curl -L -X POST 'http://localhost:8080/zone' \
-H 'Content-Type: application/json' \
--data-raw '{
    "Name": "zone-blue",
    "Description": "Tenant - Zone Blue",
    "CIDR": "10.140.0.0/20"
}'

# Create Zone - zone-red
curl -L -X POST 'http://localhost:8080/zone' \
-H 'Content-Type: application/json' \
--data-raw '{
    "Name": "zone-red",
    "Description": "Tenant - Zone Red",
    "CIDR": "172.20.0.0/20"
}'
```

- View the zones you just created:

```shell
curl -L -X GET 'http://localhost:8080/zones'
```

- Now simply join the nodes to the zones you just created. A node can only belong to one zone at a time for isolation between tenants/security zones.
- **Disclaimer:** if the zone does not exist, we do not currently handle an error channel back from the supervisor, so the agent will just sit there. Tail the supervisor logs for specifics and debugging.

```shell
# Zone Blue
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_A>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_A>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --zone=zone-blue 
    
# Zone Red
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_B>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_B>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --zone=zone-red 
```

- Once you have more than one node in a zone, the nodes can now ping one another on using the wireguard interfaces. Get the address with the following:

```shell
# Linux
ip a wg0
# OSX - Note: OSX maps wg0 to tun(n). Generally 'ifconfig utun3' will show you the specific interface
ifconfig
```

You can also view the lease state of the IPAM objects with:

```shell
curl http://localhost:8080/ipam/leases/zone-blue

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
### Child Prefixes

- Imagine a user wants to not only communicate between the node address each member of the mesh but also want to advertise
some additional IP prefixes for additional services running on a node. This can be accomplished with the `--child-prefix` flag.
Prefixes have to be unique within a zone but can overlap on separate zones.
- *Note:* once you allocate a prefix, it is fixed in IPAM. We do not currently support removing the prefix. If you want to
add different child prefix either use a different cidr or delete the persistent state file in the root of where you ran the 
supervisor binary named `<zone-name>.json`. For example, `ipam-red.json`.

```shell
# Zone Blue Node-1
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_A>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_A>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --child-prefix=172.20.1.0/24
    --zone=zone-red 

# Zone Blue Node-2
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_B>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_B>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --child-prefix=172.20.3.0/24 \
    --zone=zone-red 
```

You can create a loopback on each node's child prefix range and ping them from all nodes in the mesh like so:

```shell
# Node-1
sudo ip addr add 172.20.1.10/32 dev lo
Node-2
sudo ip addr add 172.20.3.10/32 dev lo
# ping between nodes to the loopbacks
# Both IPs are reachable because those prefixes were added to the routing tables
```

### Connect Containers Directly Between Nodes in a Mesh

The following example allows a user to connect Docker container directly to one another without exposing a port on the node.
These nodes could be in different data centers or CSPs. This example uses the `--child-prefix` option to advertise the private
container networks to the mesh and enable connectivity as depicted below.

<img src="https://jaywalking.s3.amazonaws.com/jaywalk-container-connectivity.png" width="100%" height="100%">

*Figure 2. Encrypt and connect private RFC-1918 addresses and services in containers to all nodes in the mesh regardless of location*

*Note:* the containers need to have unique private addresses on the docker network as exemplified below. Overlapping addresses 
within a zone is not supported because that is a nightmare to troubleshoot, creates major fragility in SDN deployments and is 
all around insanity. TLDR; IP address management in v4 networks is important when deploying infrastructure ¯\_(ツ)_/¯ 

- For simplicity, we are just using the default, built-in zone `default`. You can also use the zones you created in the previous exercise or create a new one.

- Node1 setup

```shell
# Node1
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_A>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_A>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --child-prefix=172.24.0.0/24 \
    --zone=default 

# Create the container network:
docker network create --driver=bridge --subnet=172.24.0.0/24 net1
# Add the address range to the wg0 interface (required for docker only):
sudo iptables -I DOCKER-USER -i wg0 -d 172.24.0.0/24 -j ACCEPT
# Start a container
docker run -it --rm --network=net1 busybox bash
```

- Node2 setup

```shell
# Node2
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_B>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_B>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --child-prefix=172.28.0.0/24 \
    --zone=default \

# Setup a docker network and start a node on it:
docker network create --driver=bridge --subnet=172.28.0.0/24 net1
# Add the address range to the wg0 interface (required for docker only):
sudo iptables -I DOCKER-USER -i wg0 -d 172.28.0.0/24 -j ACCEPT
# Start a container
docker run -it --rm --network=net1 busybox bash
# ping the container started on Node1
ping 172.28.0.x
```

- To go one step further, a user could then run jaywalk on any machine, join the mesh and ping, or connect to a service, 
on both of the containers that were started. 
- This could be a home developer's laptop, edge device, sensor or any other device with an IP address in the wild. 
- That spoke connection does not require any ports to be opened to initiate the connection into the mesh.

```shell
# Zone Blue Node3
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY_C>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY_C>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --zone=zone-blue 
    
ping 172.28.0.x
ping 172.24.0.x
```


### REST API

There are currently some supported REST calls:

- Get all peers

```shell
curl -s --location --request GET 'http://localhost:8080/peers' | python -m json.tool
```

- Get all peers output

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

- Get a peer by key

```shell
curl -s --location --request GET 'http://localhost:8080/peers/M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=' | python -m json.tool
```

- Get a peer by key output

```json
{
    "PublicKey": "M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=",
    "EndpointIP": "34.224.78.66:51820",
    "AllowedIPs": "10.20.1.1/32",
    "Zone": "zone-red"
}
```

- Get zone details

```shell
curl --location --request GET 'http://localhost:8080/zones'
```

- Zone details output (notice the overlapping CIDR address support)

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

- Get the leases of nodes in a particular zone

```shell
curl --location --request GET 'http://localhost:8080/ipam/leases/zone-blue'
curl --location --request GET 'http://localhost:8080/ipam/leases/zone-red'
```

- Lease details from a zone

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

### Developer Quickstart

- build

```shell
git clone https://github.com/redhat-et/jaywalking.git
cd jaywalking
# Default build for your OS
go build -o jaywalk
cd supervisor
go build -o jaywalk-supervisor

# Build for specific OSs
GOOS=linux GOARCH=amd64 go build -o jaywalk-amd64-linux 
GOOS=darwin GOARCH=amd64 go build -o jaywalk-amd64-darwin
```

- run

```shell
# Start the supervisor with debug logging
JAYWALK_LOG_LEVEL=debug ./jaywalk-supervisor  \
    -streamer-address <REDIS_SERVER_ADDRESS> \
    -streamer-passwd <REDIS_PASSWD>

# Start the agent on a node with debug logging
sudo JAYWALK_LOG_LEVEL=debug ./jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --zone=zone-blue 
```