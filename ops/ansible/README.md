# Ansible Deployment

There are playbooks for AWS, GCP and Azure. We recommend starting with AWS/EC2 as it is generally the easier setup of the three. The EC2 deploy also has a role for a hub router hybrid mesh demo.

## EC2 deploy

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
make run-on-kind
```

- Edit the Controller and Auth sections in `vars.yml` to add the address of the running Apex stack and change the binary address if you want a modified binary. The auth values are hardcoded as auth is still under daily development for bulk node imports such as this. The S3 bucket will generally have the latest build as we develop.

```text
### Controller Section (values are there for example, replace with your environment) ###
controller_address: <CONTROLLER_ADDRESS>
apexd_binary: https://apex-net.s3.amazonaws.com/apexd-amd64-linux
apex_zone_name: zone-hub
apex_azone_prefix: 10.185.0.0/24

### Apex Auth ###
apex_auth_uid: kitteh1@apex.local
apex_auth_password: floofykittens
apex_oidc_client_id_cli: apex-cli
apex_oidc_url: https://auth.apex.local
apex_api_url: https://api.apex.local
apex_url: https://apex.local
```

- Run the playbook (the apexd binary is stored in an S3 bucket and pulled down by ansible)

```shell
# Install Ansible if not already installed
python3 -m pip install --user ansible
ansible-playbook --version

# Run the playbook
git clone https://github.com/nexodus-io/nexodus.git
cd /apex/ops/ansible/
ansible-playbook -vv ./deploy.yml 
```

- Once the nodes are finished provisioning, ssh to the `relayNode` from the ansible inventory and run the validation test that verifies connectivity across VPCs.

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
- This will redirect to a web page to enter the pass code provided from registration (also under daily development).

```shell
sudo apexd <CONTROLLER_URL>
```

You can view the apexd logs on each deployed image with `cat ~/apex-logs.txt`

To simply stop and start the Apex agents on the nodes you can run those plays with:

```shell
# Stop the agent with:
ansible-playbook aws-apex-start.yml 

# Start the agent with:
ansible-playbook aws-apex-start.yml 
```

- Tear down the environment with:

```shell
ansible-playbook terminate-instances.yml
```
