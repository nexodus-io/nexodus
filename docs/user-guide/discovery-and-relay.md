# Deploying the Nexodus Relay Nodes

- Relay Node - Nexodus Service makes the best effort to establish a direct peering between the endpoints, but in some scenarios such as symmetric NAT, it's not possible to establish direct peering. To establish connectivity in those scenarios, Nexodus Service uses Nexodus Relay to relay the traffic between the endpoints. To use this feature you need to onboard a Relay node to the Nexodus network. This **must** be the first device to join the Nexodus network to enable the traffic relay.

Relay node needs to be reachable on a predictable Wireguard port such as 51820 and ideally at the top of your NAT cone such as running in a Cloud where all endpoints can reach relay service for peering. There is only a need for one relay node in an organization, after node joins you simply run the basic onboarding [Installing the agent](agent.md#installing-the-agent) .

## Setup Nexodus Relay Node

Clone the Nexodus repository on a VM (or bare metal machine). Nexodus relay node must be reachable from all the endpoint nodes that want to join the Nexodus network. Follow the instruction in [Starting The Agent](agent.md#starting-the-agent) section to set up the node and install the `nexd` binary.

```sh
sudo nexd --stun relay https://try.nexodus.127.0.0.1.nip.io
```

You can list the available organizations using the following command

```sh
nexctl --host https://api.try.nexodus.127.0.0.1.nip.io --username kitteh1 --password floofykittens organization list
Organization ID                          NAME      IPV4 CIDR          IPV6 CIDR     DESCRIPTION
faa76939-3226-4d09-b695-e981585ab156     kitteh1   100.100.0.0/16     200::/64      kitteh1's organization
```

### Interactive OnBoarding

```sh
sudo nexd --stun relay https://try.nexodus.127.0.0.1.nip.io
```

It will print a URL on stdout to onboard the relay node

```sh
$ sudo nexd --stun relay https://try.nexodus.127.0.0.1.nip.io
Your device must be registered with Nexodus.
Your one-time code is: GTLN-RGKP
Please open the following URL in your browser to sign in:
https://auth.try.nexodus.127.0.0.1.nip.io/device?user_code=GTLN-RGKP
```

Open the URL in your browser and provide the username and password that you used to join the node, and follow the GUI's instructions. Once you are done granting access to the device in the GUI, the relay node will be onboarded into that organization.

### Silent OnBoarding

To OnBoard devices without any browser involvement, you need to provide a username and password in the CLI command

```sh
nexd --stun --username=kitteh1 --password=floofykittens relay https://try.nexodus.127.0.0.1.nip.io
```
