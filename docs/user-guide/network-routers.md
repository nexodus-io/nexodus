# Network Routers

There may be scenarios where you do not want to run a Nexodus agent on every host that you intend to have connectivity to a Nexodus Organization. It is possible to have a host act as a Network Router to provide connectivity between a Nexodus network and a subnet a Nexodus agent node can access.

## Connect any nodes on your network or connect sites

Any Linux Nexodus device running Nexodus can be a network router. The forwarding and NAT are performed with netlink and netfilter.

The situations where this can be useful are wide-ranging. They can span from connecting a remote cluster, an air-gapped EC2 VPC, or connecting any number of remote sites that you may not want to or cannot run an agent. SD-WAN solutions can be expensive and overly complex. This can be a lightweight alternative.

- Site-A Configuration: You simply specify the network prefix you want to advertise and Nexodus will use the physical interface that contains the default route to connect to the non-Nexodus nodes.

```terminal
nexd router --child-prefix 192.168.1.0/24 --network-router --service-url https://try.nexodus.io
```

- Site-B Configuration: The only difference is specifying the network CIDR for site-B.

```terminal
nexd router --child-prefix 172.16.100.0/24 --network-router --service-url https://try.nexodus.io
```

```mermaid
graph LR
subgraph Site-B Standalone Devices not running a Nexodus Agent 172.16.100.0/24
Non_WG_Device_4["Ec2 VM1 (172.16.100.5/24)"]
Non_WG_Device_5["Ec2 VM2 (172.16.100.10/24)"]
Non_WG_Device_6["Ec2 VM3 (172.16.100.20/24)"]
Nexodus_Network_Router_B["Nexodus Network Router for Site-B
wg0-100.100.0.5 and eth0 172.16.100.50/24
(Advertising the network prefix 172.16.100.0/24)"]
end
subgraph Site-A Standalone Devices not running a Nexodus Agent 192.168.1.0/24
Non_WG_Device_1["IOT/Edge Device (192.168.1.100/24)"]
Non_WG_Device_3["Printer (192.168.1.101/24)"]
Non_WG_Device_2["Embedded Device (192.168.1.150/24)"]
Nexodus_Network_Router_A["Nexodus Network Router for Site-A
wg0-100.100.0.4 and eth0 192.168.1.10/24
(Advertising the network prefix 192.168.1.0/24)"]
end
subgraph Nexodus Peers 100.100.0.0/16
Nex_Device_1["Nexodus Peer
(wg0 100.100.0.1)"]
Nex_Device_2["Nexodus Peer
(wg0 100.100.0.2)"]
Nex_Device_3["Nexodus Peers
(wg0 100.100.0.3)"]
end
Non_WG_Device_1 -- eth0 --> Nexodus_Network_Router_A
Non_WG_Device_3 -- eth0 --> Nexodus_Network_Router_A
Non_WG_Device_2 -- eth0 --> Nexodus_Network_Router_A
Nex_Device_1 -- wg0 --> Nexodus_Network_Router_A
Nex_Device_2 -- wg0 --> Nexodus_Network_Router_A
Nex_Device_3 -- wg0 --> Nexodus_Network_Router_A
Nex_Device_1 -- wg0 --> Nexodus_Network_Router_B
Nex_Device_2 -- wg0 --> Nexodus_Network_Router_B
Nex_Device_3 -- wg0 --> Nexodus_Network_Router_B
Non_WG_Device_4 -- eth0 --> Nexodus_Network_Router_B
Non_WG_Device_5 -- eth0 --> Nexodus_Network_Router_B
Non_WG_Device_6 -- eth0 --> Nexodus_Network_Router_B
```

> **Note**
> Nexodus accepts as many networks as you want to specify in the `--child-prefix=192.168.1.0/24 --child-prefix 192.168.100.0/24 --child-prefix 172.16.100.0/24 ...` configuration. This means you can advertise as many subnets as you want from the Nexodus device running as a network router.

By default, Nexodus network routers perform NAT, specifically, source NAT for devices coming from a Nexodus mesh with a destination of one of the devices not running the Nexodus agent. This enables connectivity to those devices without any configuration on the devices.

You have the option to disable NAT with `--disable-nat` which will cause the remote non-Nexodus devices to receive traffic from the Nexodus agent devices without any address translations. This mode requires routes to be added (or redistributed in your network IGP) for hosts in `192.168.1.0/24` to reach Nexodus nodes `100.100.0.0/16` via the `Nexodus Network Router` eth0 ip of `192.168.1.10`.

The subnet exposed to the Nexodus organization may be a physical network the host is connected to, but it can also be a network local to the host. This works well for exposing a local subnet used for containers running on that host. A demo of this use case for containers can be found in [scenarios/containers-on-nodes.md](scenarios/containers-on-nodes.md).

_Additional details and diagrams are located in the network router design documentation_ [docs/development/design/network-router](https://github.com/nexodus-io/nexodus/blob/main/docs/development/design/network-router.md)
