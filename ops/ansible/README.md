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
# from the nexodus root directory run:
make run-on-kind
```

- Edit the Controller and Auth sections in `vars.yml` to add the address of the running Nexodus stack and change the binary address if you want a modified binary. The auth values are hardcoded as auth is still under daily development for bulk node imports such as this. The S3 bucket will generally have the latest build as we develop.

```text
### Controller Section (values are there for example, replace with your environment) ###
controller_address: <CONTROLLER_ADDRESS>
nexd_binary: https://nexodus-io.s3.amazonaws.com/nexd-amd64-linux
nexodus_zone_name: zone-hub
nexodus_azone_prefix: 10.185.0.0/24

### Nexodus Auth ###
nexodus_auth_uid: kitteh1@try.nexodus.127.0.0.1.nip.io
nexodus_auth_password: floofykittens
nexodus_oidc_client_id_cli: nexodus-cli
nexodus_oidc_url: https://auth.try.nexodus.127.0.0.1.nip.io
nexodus_api_url: https://api.try.nexodus.127.0.0.1.nip.io
nexodus_url: https://try.nexodus.127.0.0.1.nip.io
```

- Run the playbook (the nexd binary is stored in an S3 bucket and pulled down by ansible)

```shell
# Install Ansible if not already installed
python3 -m pip install --user ansible
ansible-playbook --version

# Run the playbook
git clone https://github.com/nexodus-io/nexodus.git
cd /nexodus/ops/ansible/
ansible-playbook -vv ./deploy.yml 
```

A good test is to initiate the test from a spoke node to make sure end-to-end traffic traverses the relay and direct peerings. Fping is also a useful utility for ping sweeping.

- Add your own machine to the mesh, for example, a mac or linux dev machine by creating a toml file of any name with your host's details:
- This will redirect to a web page to enter the pass code provided from registration (also under daily development).

```shell
sudo nexd <CONTROLLER_URL>
```

You can view the nexd logs on each deployed image with `cat ~/nexodus-logs.txt`

To simply stop and start the Nexodus agents on the nodes you can run those plays with:

```shell
# Stop the agent with:
ansible-playbook aws-nexodus-start.yml 

# Start the agent with:
ansible-playbook aws-nexodus-start.yml 
```

- Tear down the environment with:

```shell
ansible-playbook terminate-instances.yml
```
