# Deploying the Nexodus Discovery and Relay Nodes

- Relay Node - Nexodus Controller makes the best effort to establish a direct peering between the endpoints, but in some scenarios such as symmetric NAT, it's not possible to establish direct peering. To establish connectivity in those scenarios, Nexodus Controller uses Nexodus Relay to relay the traffic between the endpoints. To use this feature you need to onboard a Relay node to the Nexodus network. This **must** be the first device to join the Nexodus network to enable the traffic relay.
- Discovery Node - A Discovery Node is used to enable NAT traversal for all peers to connect to one another if they are unable to make direct connections.

Relay and Discovery can be run on the same or separate nodes. Both of these machines need to be reachable on a predictable Wireguard port such as 51820 and ideally at the top of your NAT cone such as running in a cloud where all endpoints can reach both the discovery and relay services for peering. There is only a need for one discovery and relay nodes in an organization, after those are joined you simply run the basic onboarding [Installing the agent](agent.md#installing-the-agent) .

## Setup Nexodus Relay and Discovery Node

Clone the Nexodus repository on a VM (or bare metal machine). Nexodus relay node must be reachable from all the endpoint nodes that want to join the Nexodus network. Follow the instruction in [Starting The Agent](agent.md#starting-the-agent) section to set up the node and install the `nexd` binary.

```sh
sudo nexd --discovery-node --relay-node --stun https://try.nexodus.local
```

You can list the available organizations using the following command

```sh
./nexctl  --username=kitteh1 --password=floofykittens organization list

ORGANIZATION ID                          NAME          CIDR              DESCRIPTION                RELAY/HUB ENABLED
dcab6a84-f522-4e9b-a221-8752d505fc18     default       100.100.1.0/20     Default Zone               false
```

### Interactive OnBoarding

```sh
sudo nexd --discovery-node --relay-node --stun https://try.nexodus.local
```

It will print a URL on stdout to onboard the discovery/relay node

```sh
$ sudo nexd --discovery-node --relay-node --stun https://try.nexodus.local
Your device must be registered with Nexodus.
Your one-time code is: GTLN-RGKP
Please open the following URL in your browser to sign in:
https://auth.try.nexodus.local/device?user_code=GTLN-RGKP
```

Open the URL in your browser and provide the username and password that you used to create the zone, and follow the GUI's instructions. Once you are done granting access to the device in the GUI, the relay node will be OnBoarded to the Relay Zone.

### Silent OnBoarding

To OnBoard devices without any browser involvement, you need to provide a username and password in the CLI command

```sh
nexd --discovery-node --relay-node --stun --username=kitteh1 --password=floofykittens https://try.nexodus.local
```
