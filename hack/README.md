
# Developer Quickstart

## Build the binaries
Following command will build the binaries for your default host OS, and place it in `dist` directory.

```shell
git clone https://github.com/redhat-et/apex.git
cd apex
make build-apex-local
make build-controller-local
```

TO build for specific OSs
*Linux:*
```shell
make build-apex-linux
make build-controller-linux
```
*Mac-Osx:*
```shell
make build-apex-darwin
make build-controller-darwin
```

## Setup the dev environment:
Start redis instance in Cloud or on any local node. This nodes must be reachable from all the nodes that you will be using for your dev environment to test the connectivity and the machine where Apex Controller will be running (e.g your laptop). Below is an example for podman or docker for ease of use, no other configuration is required.

```shell
docker run \
    --name redis \
    -d -p 6379:6379 \
    redis redis-server \
    --requirepass <REDIS_PASSWD>
```

You can verify that the redis server is running using following command:
```shell
docker run -it --rm redis redis-cli -h <container-host-ip> -a <REDIS_PASSWD> --no-auth-warning PING
```
If it outputs **PONG**, that's a success.

Start the Apex Controller with debug logging (You can start it on your laptop where you are hacking). It contains two component 
1) postgres db instance
```shell
docker run --name postgres -e POSTGRES_USER=<USERNAME> -e POSTGRES_PASSWORD=<PASSWORD> -p 5432:5432 -d postgres
```

2) controller instance (connects to postgres db instance for persistent storage)
```shell
CONTROLLER_LOG_LEVEL=debug ./controller  \
    --streamer-address <REDIS_SERVER_ADDRESS> \
    --streamer-password <REDIS_PASSWD> \
    --db-address <POSTGRES_ADDR> \
    --db-password <POSTGRES_PASS>    

```

Start the agent on a node with debug logging. This is just an example command which starts the agent and connect the node to default zone. If you are testing different connectivity scenarios as mentioned in the main [readme](../README.md), you need to invoke the agent with relevant configuration options:
```shell
sudo APEX_LOG_LEVEL=debug ./apex --public-key=<NODE_WIREGUARD_PUBLIC_KEY>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --zone=default 
```

##  Cleanup the dev environment
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

Kill your Controller and build & restart if you want to deploy new changes.