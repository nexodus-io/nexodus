# Nexodus Node Agent `nexd`

## Overview

`nexd` implements a node agent to configure encrypted mesh networking on your device with nexodus.

<!--  everything after this comment is generated with: ./hack/nexd-docs.sh -->
### Usage

```text
NAME:
   nexd - Node agent to configure encrypted mesh networking with nexodus.

USAGE:
   nexd [global options] command [command options] [arguments...]

COMMANDS:
   version  Get the version of nexd
   proxy    Run nexd as an L4 proxy instead of creating a network interface
   router   Enable child-prefix function of the node agent to enable prefix forwarding.
   relay    Enable relay and discovery support function for the node agent.
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h           Show help
   --unix-socket value  Path to the unix socket nexd is listening against (default: "/Users/chirino/.nexodus/nexd.sock")

   Agent Options

   --relay-only  Set if this node is unable to NAT hole punch or you do not want to fully mesh (Nexodus will set this automatically if symmetric NAT is detected) (default: false) [$NEXD_RELAY_ONLY]
   --stun        Discover the public address for this host using STUN (default: false) [$NEXD_STUN]

   Nexodus Service Options

   --insecure-skip-tls-verify                   If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure (default: false) [$NEXD_INSECURE_SKIP_TLS_VERIFY]
   --organization-id value                      Organization ID to use when registering with the nexodus service [$NEXD_ORG_ID]
   --password string                            Password string for accessing the nexodus service [$NEXD_PASSWORD]
   --service-url value                          URL to the Nexodus service (default: "https://try.nexodus.127.0.0.1.nip.io") [$NEXD_SERVICE_URL]
   --state-dir value                            Directory to store state in, such as api tokens to reuse after interactive login. Defaults to'/Users/chirino/.nexodus' (default: "/Users/chirino/.nexodus") [$NEXD_STATE_DIR]
   --stun-server value [ --stun-server value ]  stun server to use discover our endpoint address.  At least two are required. [$NEXD_STUN_SERVER]
   --username string                            Username string for accessing the nexodus service [$NEXD_USERNAME]

   Wireguard Options

   --listen-port port      Wireguard port to listen on for incoming peers (default: 0) [$NEXD_LISTEN_PORT]
   --local-endpoint-ip IP  Specify the endpoint IP address of this node instead of being discovered (optional) [$NEXD_LOCAL_ENDPOINT_IP]
   --request-ip IPv4       Request a specific IPv4 address from IPAM if available (optional) [$NEXD_REQUESTED_IP]

```

#### nexd proxy

```text
NAME:
   nexd proxy - Run nexd as an L4 proxy instead of creating a network interface

USAGE:
   nexd proxy [command options] [arguments...]

OPTIONS:
   --ingress value [ --ingress value ]  Forward connections from the Nexodus network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via a locally accessible network using a value in the form: protocol:port:destination_ip:destination_port. All fields are required.
   --egress value [ --egress value ]    Forward connections from a locally accessible network made to [port] on this proxy instance to port [destination_port] at [destination_ip] via the Nexodus network using a value in the form: protocol:port:destination_ip:destination_port. All fields are required.
   --help, -h                           Show help
```

#### nexd router

```text
NAME:
   nexd router - Enable child-prefix function of the node agent to enable prefix forwarding.

USAGE:
   nexd router [command options] [arguments...]

OPTIONS:
   --child-prefix CIDR [ --child-prefix CIDR ]  Request a CIDR range of addresses that will be advertised from this node (optional) [$NEXD_REQUESTED_CHILD_PREFIX]
   --network-router                             Make the node a network router node that will forward traffic specified by --child-prefix through the physical interface that contains the default gateway (default: false) [$NEXD_NET_ROUTER_NODE]
   --disable-nat                                disable NAT for the network router mode. This will require devices on the network to be configured with an ip route (default: false) [$NEXD_DISABLE_NAT]
   --help, -h                                   Show help
```

#### nexd relay

```text
NAME:
   nexd relay - Enable relay and discovery support function for the node agent.

USAGE:
   nexd relay [command options] [arguments...]

OPTIONS:
   --help, -h  Show help
```
