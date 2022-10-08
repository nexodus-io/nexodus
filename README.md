# Jaywalking

[![build](https://github.com/redhat-et/jaywalking/actions/workflows/build.yml/badge.svg)](https://github.com/redhat-et/jaywalking/actions/workflows/build.yml)

Roads? Where we're going, we don't need roads - *Dr Emmett Brown*

### Jaywalk Quickstart

<img src="https://jaywalking.s3.amazonaws.com/jaywalker-multi-tenant.png" width="100%" height="100%">

*Figure 1. Getting started topology that can be setup in minutes.* 

- Build for the node OS that is getting onboarded to the mesh:

```
GOOS=linux GOARCH=amd64 go build -o jaywalk-amd64-linux
GOOS=darwin GOARCH=amd64 go build -o jaywalk-amd64-darwin
# Windows support soon.

# Or download a recent build to the nodes you are looking to add to the mesh:
# OSX Binary
curl https://jaywalking.s3.amazonaws.com/jaywalk-amd64-darwin --output /usr/local/sbin/jaywalk
chmod +x /usr/local/sbin/jaywalk
# Linux Binary
sudo curl https://jaywalking.s3.amazonaws.com/jaywalk-amd64-linux --output /usr/local/sbin/jaywalk
chmod +x /usr/local/sbin/jaywalk
```

- Start a redis instance in EC2 or somewhere all nodes can reach (below is an example for podman or docker for ease of use, no other configuration is required):

```
docker run \
    --name redis \
    -d -p 6379:6379 \
    redis redis-server \
    --requirepass <REDIS_PASSWD>
```

- Start the supervisor/controller SaaS portion (this can be your laptop, the only requirement is it can reach the redis streamer started above):

```
git clone https://github.com/redhat-et/jaywalking.git

cd supervisor
go build -o jaywalk-supervisor

./jaywalk-supervisor \
    -streamer-address <REDIS_SERVER_ADDRESS> \
    -streamer-passwd <REDIS_PASSWD>
```

- Generate your private/public key pair:

```
# For a Linux node run the following. For Windows and Mac adjust the paths to existing directories. ex. ~/.wireguard/
wg genkey | sudo tee /etc/wireguard/server_private.key | wg pubkey | sudo tee /etc/wireguard/server_public.key
```

- Start the jaywalk agent on the node you want to join the mesh and fill in the relevant configuration. IP addressing of the mesh network is managed via the controller:

```
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
     --agent-mode
```

- You will now have a flat host routed network between the endpoints. We currently work around NAT with a STUN server to automatically discover public addressing for the user.

- Cleanup

```
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
- This also provides multi-tenancy and overlapping CIDR IPv4 or IPv6 by providing the `--zone=zone-blue` or `--zone=zone-red`. These will be made more generic moving forward.
- You can also run the jaywalk command on one node and then run the exact same command and keys on a new node and the assigned address from the supervisor will move that peering
  from to the new machine you run it on along with updating the mesh as to the new endpoint address.
- This can be run behind natted networks for remote spoke machines and do not require any incoming ports to be opened to the device. Only one side of the peering needs an open port
  for connections to be initiated. Once the connection is initiated from one side, bi-directional communications can be established. This aspect is especially interesting for IOT/Edge.
- An IPAM module handles node address allocations but also allows the user to specify it's wireguard node address.


Another join example from a node includes the ability to specify what zone to join and allows you to request a particular IP address from the IPAM module. If an existing lease exists, it
will be released and offered to the node requesting it.

```curl
sudo jaywalk --public-key=<NODE_WIREGUARD_PUBLIC_KEY>  \
    --private-key=<NODE_WIREGUARD_PRIVATE_KEY>  \
    --controller=<REDIS_SERVER_ADDRESS> \
    --controller-password=<REDIS_PASSWORD> \
    --agent-mode \
    --request-ip=10.10.0.30 \
    --zone=zone-red 
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
"AllowedIPs": "10.10.1.1/32",
"Zone": "zone-blue"
},
{
"PublicKey": "O3UVnLl6BFNYWf21tEDGpKbxYfzCp9LzwSXbtd9i+Eg=",
"EndpointIP": "18.205.149.74:51820",
"AllowedIPs": "10.10.1.2/32",
"Zone": "zone-blue"
},
{
"PublicKey": "SvAAJctGA5U6EP+30LMuhoG76VLrEhwq3rwFf9pqcB4=",
"EndpointIP": "3.82.51.92:51820",
"AllowedIPs": "10.10.1.3/32",
"Zone": "zone-blue"
},
{
"PublicKey": "M+BTP8LbMikKLufoTTI7tPL5Jf3SHhNki6SXEXa5Uic=",
"EndpointIP": "34.224.78.66:51820",
"AllowedIPs": "10.10.1.1/32",
"Zone": "zone-red"
},
{
"PublicKey": "oJlDE1y9xxmR6CIEYCSJAN+8b/RK73TpBYixlFiBJDM=",
"EndpointIP": "71.31.21.22:51820",
"AllowedIPs": "10.10.1.2/32",
"Zone": "zone-red"
},
{
"PublicKey": "IMqxPz/eQzCdHjb8Ajl7OVTtJmZqiKeS6SvQLml21nU=",
"EndpointIP": "71.31.21.22:51820",
"AllowedIPs": "10.10.1.3/32",
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
    "AllowedIPs": "10.10.1.1/32",
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
    "IpCidr": "10.10.1.0/20"
  },
  {
    "Name": "zone-blue",
    "Description": "Tenancy Zone Blue",
    "IpCidr": "10.10.1.0/20"
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
        "Cidr": "10.10.0.0/20",
        "IPs": {
            "10.10.0.0": true,
            "10.10.0.1": true,
            "10.10.0.2": true,
            "10.10.0.29": true,
            "10.10.0.3": true,
            "10.10.0.4": true,
            "10.10.0.5": true,
            "10.10.0.6": true,
            "10.10.15.255": true
        }
    }
]
```



### Ansible Deployment

To deploy the current state run the following which deploys nodes across two VPCs and enables full mesh connectivity between them (simulating two disparate data centers)

- Setup your aws profile with the required keys for ec2 provisioning:

```shell
vi ~/.aws/credentials
[default]
region = us-east-1
aws_access_key_id = <aws_access_key_id>
aws_secret_access_key = <aws_secret_access_key>
```

- Edit the controller section to add the running redis server address/password and change the binary address if you want a modified binary. The S3 bucket will generally have the latest build as we develop.

```
### Controller Section ###
controller_address: <ADD REDIS ADDRESS HERE>
controller_password: <ADD PASS HERE>
jaywalk_binary: https://jaywalking.s3.amazonaws.com/jaywalk-zeroconf-poc-amd64-linux
```
- Run the playbook (the jaywalk binary is stored in an S3 bucket and pulled down by ansible)

```shell
# Install Ansible if not already installed
python3 -m pip install --user ansible
ansible-playbook --version

# Run the playbook
git clone https://github.com/redhat-et/jaywalking.git
cd /jaywalking/ops/ansible/
ansible-playbook -vv ./deploy.yml 
```

- Once the nodes are finished provisioning, ssh to a node from the inventory and run the validation test that verifies connectivity across VPCs. 

```shell
cat inventory.txt
ssh -i <key_name>.pem ubuntu@<ip_from_inventory>

./verify-connectivity.sh
node 10.10.1.8 is up
node 10.10.1.7 is up
node 10.10.1.6 is up
node 10.10.1.5 is up
node 10.10.1.4 is up
node 10.10.1.3 is up
node 10.10.1.2 is up
node 10.10.1.1 is up
```

- Add your own machine to the mesh, for example, a mac or linux dev machine by creating a toml file of any name with your host's details:

```
sudo jaywalk --public-key=<PUBLIC_KEY> \
    --private-key=<PRIVATE_KEY>  \
    --controller=<CONTROLLER_ADDRESS>  \
    --controller-password=<CONTROLLER_ADDRESS> \
    --agent-mode
```

- Copy the toml file into the `ops/ansible/peer-inventory` directory and re-run the playbook.

- Now your host should be able to reach all nodes in the mesh, verify with `./verify-connectivity.sh` located in the ansible directory.

- Tear down the environment with:
```
ansible-playbook terminate-instances.yml
```
