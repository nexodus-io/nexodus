
# Ansible Deployment

**Note:** Maintenance of the playbooks will be sporadic as we focus on the controller aspect of the project. Once CI is leveraging these it will improve.

To deploy the current state run the following which deploys nodes across two VPCs and enables full mesh connectivity between them (simulating two disparate data centers)

- Setup your aws profile with the required keys for ec2 provisioning:

```shell
vi ~/.aws/credentials
[default]
region = us-east-1
aws_access_key_id = <aws_access_key_id>
aws_secret_access_key = <aws_secret_access_key>
```

- Start the controller on a node

```shell
# from the apex root directory run:
docker-compose up
```

- Get a token from the controller

```shell
cd hack
./keycloak-grant.sh admin floofykittens
```

- Provision a hub-zone and copy the UUID returned for the zone for the next step

```curl
curl --location --request POST 'http://<CONTROLLER_IP_COMPOSE_STACK>:8080/zones' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer <OUTPUT OF BEARER TOKEN FROM THE PREVIOUS STEP>' \
--data-raw '{
    "name": "zone-hub",
    "description": "Hub/Spoke Zone",
    "cidr": "10.185.0.0/20",
    "hub-zone": true
}'
```

- Edit the controller section to add the running redis server address/password and change the binary address if you want a modified binary. The S3 bucket will generally have the latest build as we develop.

```
### Controller Section ###
controller_address: <ADD REDIS ADDRESS HERE>
controller_password: <ADD PASS HERE>
apex_binary: https://jaywalking.s3.amazonaws.com/jaywalk-zeroconf-poc-amd64-linux
apex_zone_uuid: <ZONE_UUID>
```

- Run the playbook (the apex binary is stored in an S3 bucket and pulled down by ansible)

```shell
# Install Ansible if not already installed
python3 -m pip install --user ansible
ansible-playbook --version

# Run the playbook
git clone https://github.com/redhat-et/apex.git
cd /apex/ops/ansible/
ansible-playbook -vv ./deploy.yml 
```

- Once the nodes are finished provisioning, ssh to the `hubRouterNode` from the ansible inventory and run the validation test that verifies connectivity across VPCs.

```shell
ssh -i <key_name>.pem ubuntu@<ip_from_inventory>

# copy the node addresses to a file
sudo grep 10.180.0 /etc/wireguard/wg0.conf | awk '{print $3}'

./verify-connectivity.sh <name of file you copied the node addresses to>
node 10.180.0.6 is up
node 10.180.0.2 is up
node 10.180.0.3 is up
node 10.180.0.4 is up
node 10.180.0.5 is up
...

# The best test is to initiate the test from a spoke node to make sure end-to-end traffic traverses the hub and direct peerings
```

- Add your own machine to the mesh, for example, a mac or linux dev machine by creating a toml file of any name with your host's details:

```
sudo apex --public-key=<PUBLIC_KEY> \
    --private-key=<PRIVATE_KEY>  \
    --controller=<CONTROLLER_ADDRESS>  \
    --controller-password=<CONTROLLER_ADDRESS>
    --zone=<ZONE_UUID>
```

- Copy the toml file into the `ops/ansible/peer-inventory` directory and re-run the playbook.

- Now your host should be able to reach all nodes in the mesh, verify with `./verify-connectivity.sh` located in the ansible directory.

- Teardown the environment with:
```
ansible-playbook terminate-instances.yml
```
