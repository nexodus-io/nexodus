
# Developer Quickstart

## Dev Environment setup using docker-compose

### On local machine
Clone the apex repo:

```shell
git clone https://github.com/redhat-et/apex.git
```

```shell
cd apex
docker-compose build
docker-compose up -d
```

These commands will build all the required binaries/images and start all the component of the controller stack (redis server, postgres db, keycloak instance, UI and controller).

If you already have a running redis server instance and wants to use that, you can specify following environment variable to point the stack to that remote redis server

```shell
STREAMER_IP=<redis-ip-address> STREAMER_PASSWORD=<redis-password> docker-compose up -d
```

Similarly if want to connect to any existing postgres instance
```shell
DB_IP=<db-ip> DB_USER=<db-user-name> DB_PASSWORD=<db-password> docker-compose up -d
```

### On Remote Machine hosted somewhere in your intranet)
Make sure the remote machine have key based authentication enabled for ssh (won't work with password prompts), and also running docker and docker-compose.

```shell
docker context create remote --docker "host=ssh://<REMOTE_USER>@<REMOTE_MACHINE_IP_OR_DNS"

docker-compose --context remote build
docker-compose --context remote up -d
```

### On Remote Machine hosted on Cloud (E.g Aws)

Add the access ID of the Cloud VM to your local machine `~/.ssh/config`.
```
Host <dns-name-of-cloud-vm>
  User <USER>
  IdentityFile <Location of the access key>

e.g 
Host ec2-xxx-yyy-zzz-www.us-west-1.compute.amazonaws.com
  User ubuntu
  IdentityFile ~/Downloads/aws/aws-access-key.pem
```

That should allow you to access your Cloud VM without explicitly specifying the access key using `-i` option.
Once you setup key base authentication for your Cloud VM, you can create a docker context and use that for docker-compose.

```shell
docker context create cloudvm --docker "host=ssh://<USER>@<CLOUD_VM_IP_OR_DNS"

docker-compose --context cloudvm build
docker-compose --context cloudvm up -d
```

### On Kubernetes deployment 
You will need the Kubernetes config file available somewhere locally to deploy the stack on the kubernetes cluster using docker-compose.

Create a new context 

```shell
docker context create k8s \
    --default-stack-orchestrator=kubernetes \
    --kubernetes config-file=<path-to-kube-config> \
    --docker host=unix:///var/run/docker.sock
```

and use the context with docker-compose to deploy controller stack

```shell
docker-compose --k8s cloudvm build
docker-compose --k8s cloudvm up -d
```

## Dev environment setup using individual components

### Build the binaries
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

### Setup the dev environment:
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

### Controller cleanup
If you are using docker-compose, run `docker compose down` and it will remove all the containers running the controller stack.

If you have setup individual components, kill those processes.

### Node cleanup
If you want to remove the node from the network, and want to cleanup all the configuration done on the node. Fire away following commands:

Ctrl + c (cmd+c) the Apex process, and remove the wireguard interface and relevant configuration files.
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