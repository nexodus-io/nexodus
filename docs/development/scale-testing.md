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

To run the tests, simply go to the actions and choose the deployment size: small, medium, large, or xlarge. The node counts of these will change as we optimize the deployment. Next, choose the pull request number or branch name. This value would simply be a number or something like main. From there, you can watch the logs and troubleshoot any errors as part of debugging a PR, download logs from the artifacts zip at the end of the workflow, or see the successful or unsuccessful connectivity results in the Mesh Connectivity Results step of the workflow.

Once the workflow completes, all the EC2 agent instances that were started are torn down, based on the EC2 tag set in the Ansible variables.
