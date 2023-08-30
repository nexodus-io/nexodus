# Mesh network between containers running on connected nodes

Imagine a user wants to not only communicate between the node address each member of the mesh but also want to advertise
some additional IP prefixes for additional services running on a node. This can be accomplished with the `--advertise-cidr` flag
of `router` subcommand. Prefixes have to be unique within an organization but can overlap between separate organizations.

The following example allows a user to connect Docker container directly to one another without exposing a port on the node.
These nodes could be in different data centers or CSPs. This example uses the `router --advertise-cidr` option to advertise the private
container networks to the mesh and enable connectivity.

**Node1 setup:**

Join node1 to the user's default assigned organization

```shell
sudo nexd router --advertise-cidr=172.24.0.0/24 <SERVICE_URL>
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

Join node2 to the user's default assigned organization

```shell
sudo nexd router --advertise-cidr=172.28.0.0/24 <SERVICE_URL>
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

To go one step further, a user could then run nexd on any machine, join the mesh and ping, or connect to a service, on both of the containers that were started. This could be a home developer's laptop, edge device, sensor or any other device with an IP address in the wild. That spoke connection does not require any ports to be opened to initiate the connection into the mesh.

```shell
sudo nexd <SERVICE_URL>
```

Ping to prefixes on both the other nodes should be successful now.

```sh
ping 172.28.0.x
ping 172.24.0.x
```

**NOTES:**

- once you allocate a prefix, it is fixed in IPAM. We do not currently support removing the prefix.

- Containers need to have unique private addresses on the docker network as exemplified above. Overlapping addresses
within an organization is not supported. However, IPAM is namespaced to support overlapping addresses between organizations similar to for example, VPCs on EC2.
