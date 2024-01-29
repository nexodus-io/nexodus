# Nexodus Scale Testing

## Overview

Scale testing involves running Ansible playbooks to deploy nodes that then attach to a Nexodus api-server deployment. The in-tree playbooks for running the tests can be found in [ops/ansible/](../../ops/ansible/).

There are two workflows that can be used to measure scale and performance. The agent side is the same for both tests.

- Agent nodes are deployed to EC2 and attach to the specified controller.
- Ansible variables that set the node counts, api-server URLs, and EC2 details such as VPCs and security groups are stored in the repo's Github Secrets.
- Agents are deployed across three different VPCs with no inbound rules except for the relay node, which requires port exposure for devices behind NATs incapable of traversal.
- Playbooks are available for EC2, GCP, and Azure. Although these are occasionally validated, there is no difference with regard to NAT traversal, which is the primary validation.
- Once agents have been attached, connectivity is verified and reported back to the Github runner for output.
- Logs from all nodes are collected and uploaded to artifacts for postmortem analysis in the case of failures.

### Agent Side Deployment

Both scale workflow scenarios deploy agents in the same manner. There are three different profiles in which an agent can be launched. All three are tested simultaneously to validate interconnectivity between profiles:

1. Numerous Nexodus agents running with default configurations. There are no firewall rules open to the nodes.
2. One Nexodus relay node. The relay node handles connections for agent nodes that cannot perform NAT hole punching to open a NAT binding for other peers to connect to. This node has its listening port exposed to the internet to accept connections.
3. Numerous relay-only nodes. These are started with the flag `--relay-only`, which sets the agent's mode to symmetric NAT, meaning it only peers to the relay node and not to any other agents. This is a traditional hub-and-spoke model that is being validated to ensure that connectivity can be established to all nodes by traversing the relay node, which has full mesh peering.

### Workflow `qa-dev`

- This workflow is ideal for testing any PR for functionality not covered in e2e. Mocking up complex networks in runners with containers can quickly turn into spaghetti. This tests real-world NAT traversal with virtually no EC2 cost incurred with micro nodes. That said, the controller is currently running in a Kind deployment on a relatively small EC2 node (t2.xlarge), so it's likely to peak much sooner than the QA/prod environments. It is still very valuable to run potentially disruptive PRs through this test before merging.

- [qa-dev](https://github.com/nexodus-io/nexodus/actions/workflows/qa-dev.yml) deploys both the api-server stack and the agent. This workflow clones either main or the specified PR number to a Kind stack running in an EC2 instance. Since this is self-signed, the playbooks have to take into account adding the certs and building and deploying the api-server. At the start of each deployment of this workflow, the images are built and loaded into Kind. Next, the kube deployments are restarted, and a database wipe is performed. From there, agent nodes are deployed with the rootCA of the controller and attached and validated. api-server logs are also gathered as part of this workflow.

### Workflow `qa-scale`

This workflow is for validating the production environment. The api-server/controller is already running an instance of what is in main. The workflow creates new user IDs for the workflow being run since the database is not being wiped before or after the workflow like it is in qa-dev.

- [qa-scale](https://github.com/nexodus-io/nexodus/actions/workflows/qa-scale.yml) deploys the agents against the QA deployment. This deployment is a mirror of the production deployment in a different namespace. Code is validated in QA and then promoted to production. There is no option to deploy a particular PR to QA, it is running the latest main branch. Keep this in mind if you are picking a PR to test that interoperates with the current QA deployment.

## Running the Tests

To run the tests, simply go to the actions and choose the deployment size: small, medium, large, or xlarge. The node counts of these will change as we optimize the deployment. Next, choose the pull request number or branch name. This value would simply be a number or a branch such as main. From there, you can watch the logs and troubleshoot any errors as part of debugging a PR, download logs from the artifacts zip at the end of the workflow, or see the successful or unsuccessful connectivity results in the Mesh Connectivity Results step of the workflow.

Once the workflow completes, all the EC2 agent instances that were started are torn down, based on the EC2 tag set in the Ansible variables.

### Additional Dispatch Arguments

There are additional arguments that can be passed via the Github Actions dispatch that allows for adjusting the following values.

- EC2 Instance Type: The options are t2.micro, t2.small, t2.medium, or t2.large. The default is t2.micro.
- Pull Request Number or Branch Name: This defaults to "main" branch. Enter a PR number if you want to test a patch instead of main.
- Time in Minutes to Pause Before Tearing Down the Infrastructure for Debugging: This is set to 0 minutes by default, meaning there will be no pause.
- Timeout in Minutes for the Deploy-QA Job: The job will time out after 90 minutes by default. Adjust this to a longer period in conjunction with the pause timer for long-running debugging sessions.
- Containers Per Node: By default, it is 5 containers per node. If you specify a high number of containers you would likely want to bump the instance type to `t2.medium` or `t2.large`. This option is only available for the `qa-scale-containers` workflow.

### Scale Test Attaching Containers Outside of CI to QA Servers

You can use the script [nexodus/hack/e2e-scripts/scale/qa-container-scale.sh](../../hack/e2e-scripts/scale/qa-container-scale.sh) to launch containers and attach them to a Nexodus api-server for scale testing.

Prerequisites are having Go and Docker installed.

Run the script with the following arguments.

```text
git clone https://github.com/nexodus-io/nexodus.git
cd nexodus/hack/e2e-scripts/scale
./qa-container-scale.sh --kc-password "<ADMIN_KEYCLOAK_PASSWORD>" --nexd-password "<PASS_CAN_BE_ANYTHING>" --org-count 1 --nexd-count 3
```

Connect to the containers after the script is run.

```text
docker exec -it <CID> bash
```

Once on a container, verify connectivity.

```text
nexctl nexd peers ping
```

Once done, cleanup the container individually or delete all running containers. Note, this will delete ALL running containers.

```text
docker rm -f $(docker ps -a -q)
```

The script assumes the user can run docker by adding the current user to the docker group.

```text
sudo groupadd docker
sudo usermod -aG docker $USER
```

### Scale Testing Attaching to a KIND Deployment

If you want to run scale testing via containers outside of CI similar to the last example but to a KIND server, you can use the  [nexodus/hack/e2e-scripts/qa-kind-container-scale.sh](../../hack/e2e-scripts/qa-kind-container-scale.sh)

```text
git clone https://github.com/nexodus-io/nexodus.git
cd nexodus/hack/e2e-scripts
/qa-kind-container-scale.sh  --nexd-user kitteh1 --nexd-password "floofykittens" --nexd-count 3 --api-server-ip x.x.x.x
```

To use Podman as the container runtime, simply add the `--podman` flag.

```text
git clone https://github.com/nexodus-io/nexodus.git
cd nexodus/hack/e2e-scripts
/qa-kind-container-scale.sh --podman --nexd-user kitteh1 --nexd-password "floofykittens" --nexd-count 3 --api-server-ip x.x.x.x
```

The difference is this script adds the self-signed cert from the kind deployment, that is created with the `make ca-cert` in the KIND configuration. The cert has to be in the same directory as the script in a file named `rootCA.pem` in order to be copied to each container for import.

## Nexodus Control Plane Scale Testing Results (as of Oct, 2023)

### High level scale results

- Nexodus Control Plane can scale to 10000 devices with 200 organization with 50 devices per organization.
- Nexodus Control Plane can scale to 1000 devices per organization.

### Scale testing setup

- Nexodus control plane stack was replicated into a playground namespace in the same cluster.
- Using the [qa-container-scale.sh](../../hack/e2e-scripts/scale/qa-container-scale.sh) script, multiple devices running the nexd agent were created and connected to the Nexodus control plane.
- Used AWS EC2 x2large instance to spawn these nexd devices. Each EC2 instance was able to accommodate 1000 containers. A New EC2 VM was added as we need to scale more.
- Helpful scripts to run scale tests are available in [nexodus/hack/e2e-scripts/scale](../../hack/e2e-scripts/scale) directory.

### Major challenges for scale

- High CPU of some key components - ApiServer and ApiProxy. A major contributor to the high CPU were the slow DB queries (such as fetching devices, security groups). Because of the slow DB queries, ApiServer was not able to process the requests fast enough and new device onboarding started failing.
  - Implementing watchers for the slow queries and optimizing the queries help reduce the overall request processing time and the CPU usage.
- ApiProxy to ApiServer connection scale issues. As the number of devices increased, the number of connections & requests per connection to ApiProxy (Envoy proxy) to ApiServer increased. This caused the ApiProxy to get overloaded and that triggered envoy proxy's circuit breakers. Due to the open circuit breaker, nexd agent started seeing request failures and stopped getting updates from the control plane.
  - Configuring the envoy proxy's connection pool and circuit breaker to handle the 10K scale, resolved the connection scale issues.
- ApiServer to DB connection scale issues. Once the ApiProxy to ApiServer connection/request rate limit was fixed, it allowed a higher number of connections/requests from ApiServer to DB as the number of devices increased. This caused the DB to get overloaded and DB start running out of connections.
  - Introduced PGBouncer between the ApiServer and DB to manage connection pooling configuration and DB connection configuration (shared buffer, max connections, max client connections)which resolved the connection issues.
- Once the above mentioned issues were resolved, the ApiProxy, ApiServer and DB pods start encountering high CPU, but this time it was simply because they needed more compute to process more requests. At this point, scale was proportioned to the available compute to these components.
  - Horizontally scaling the replicas and allocating more compute resources to these replicas handled 10K devices with 60% overall resources usage. Refer to this [deployment yaml](https://github.com/nexodus-io/nexodus/blob/main/deploy/nexodus/overlays/playground/kustomization.yaml#L91) for more details about the replicas and compute resources allocated to each component.
