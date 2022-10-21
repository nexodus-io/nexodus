
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

- Edit the controller section to add the running redis server address/password and change the binary address if you want a modified binary. The S3 bucket will generally have the latest build as we develop.

```
### Controller Section ###
controller_address: <ADD REDIS ADDRESS HERE>
controller_password: <ADD PASS HERE>
apex_binary: https://jaywalking.s3.amazonaws.com/jaywalk-zeroconf-poc-amd64-linux
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
sudo apex --public-key=<PUBLIC_KEY> \
    --private-key=<PRIVATE_KEY>  \
    --controller=<CONTROLLER_ADDRESS>  \
    --controller-password=<CONTROLLER_ADDRESS>
```

- Copy the toml file into the `ops/ansible/peer-inventory` directory and re-run the playbook.

- Now your host should be able to reach all nodes in the mesh, verify with `./verify-connectivity.sh` located in the ansible directory.

- Tear down the environment with:
```
ansible-playbook terminate-instances.yml
```
