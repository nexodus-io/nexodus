# Discovery Design

Discovery is a key component of Nexodus. Enterprise workloads are spread across all manner of networks both managed and unmanaged. Even networks under the same administrative domain typically require manual intervention or approval to enable direct access between workloads. One of this project's goals is for endpoints to not require custom firewall rules or static NAT mappings to have continuous connectivity.

- Nexodus and, in general, most Wireguard-based projects are leveraging a decade worth of discovery and connectivity mechanisms that evolved out of VOIP and media streaming needs.
- The general standard protocol for NAT traversal and peer candidate discovery that we plan to follow is laid out in [RFC8445 Interactive Connectivity Establishment (ICE): A Protocol for Network Address Translator (NAT) Traversal](https://www.rfc-editor.org/rfc/rfc8445).
- The goal is to create direct peering between nodes where possible and bounce connections through a relay node where direct connections are not possible. Nexodus should be able to set up direct peering in most cases, even when two endpoints are not ordinarily able to reach each other directly.

```text
                      To Internet

                          |
                          |
                          |  /------------  Relayed
                      Y:y | /               Address
                      +--------+
                      |        |
                      |  TURN  |
                      | Server |
                      |        |
                      +--------+
                          |
                          |
                          | /------------  Server
                   X1':x1'|/               Reflexive
                    +------------+         Address
                    |    NAT     |
                    +------------+
                          |
                          | /------------  Local
                      X:x |/               Address
                      +--------+
                      |        |
                      | Agent  |
                      |        |
                      +--------+

```

*Figure 1. Candidate Types*

## Current and Future Discovery Plans

> **Warning**
> Current peer discovery is still in an early POC state. All discovery scenarios are subject to change.

### Nodes without a firewall and/or NAT device between them (currently supported)

- Nexodus will peer end nodes directly that share the same reflexive address discovered via a STUN server.
- This means that when both nodes share the same public or "reflexive address" (as defined in the above figure), Nexodus assumes that other peers are likely to have direct access to one another. Each peer has their stun/reflexive address in the peer listing received from the controller.
- Next, the Nexodus agent will look up the "Local Address" (see Figure 1) of a peer candidate in the peer listing and attempt to probe for connectivity. If this probing succeeds, we consider it a likely candidate match, and both peers set up the connection to one another with a /32 host route and wireguard tunnel.

### Nodes with a firewall and/or NAT device between them (currently supported)

```text
                               +---------+
             +--------+        |  Relay  |        +--------+
             | STUN   |        |  Server |        | STUN   |
             | Server |        +---------+        | Server |
             +--------+       /           \       +--------+
                             /             \
                            /               \
                           /                 \
                          /                   \
                   +--------+                +--------+
                   |  NAT   |  <- Direct->   |  NAT   |
                   +--------+     Peering    +--------+
                      /                             \
                     /                               \
                 +-------+                       +-------+
                 | Agent | Encrypted Connection  | Agent |
                 |   L   | ====================  |   R   |
                 +-------+                       +-------+
```

*Figure 2. NAT traversal connecting directly via reflexive addresses sockets learned via STUN*

- Current discovery of valid sockets is gathered from a relay node that all peers can attach to. That state is distributed to all peer nodes (except for symmetric NAT nodes that will use the relay to reach all other nodes).
- Two endpoints that do not match the initial local address peering, will attempt to peer via the STUN method (see the ICE RFC or the STUN [RFC3489](https://www.ietf.org/rfc/rfc3489.txt) for further details regarding the STUN protocol). A STUN request can be used to open a source UDP port on the NAT device front-ending the endpoint.
- A STUN server allows nodes to discover their public address, the type of NAT they are behind and the Internet side port associated by the NAT with a particular local port. This information is used to set up a UDP connection between the two peers behind the NAT device.
- That UDP source port will remain open for some unknown period of time (depending on the NAT device).
- Next we attempt to match peer candidates with that IP:Port STUN address for the endpoint.
- All nodes will attach to a relay node running a STUN server and/or terminating wireguard peerings for fallback forwarding in the case a peer candidate match cannot be made. This peering can be reconciled from wireguard state until we no longer terminate peers on the relay node (tbd).
- This peering can handle multiple nodes behind a PAT (port address translation) device since there will always be a unique source address assigned in the connection state.
- How much of the discovery can be done solely by the agents without interaction from the relay node is also something being investigated. Ideally agents can negotiate the bulk of peer candidacy for scaling reasons.

### Nodes that still cannot find a match (currently supported)

- For all nodes that are unable to find a suitable candidate peering, we fall back to forwarding traffic through the relay node that needs to be open (UDP 51820) to all endpoints in the zone.
- Currently, all nodes that peer into a zone will peer to the relay node to ensure connectivity in all scenarios. Since this is a hairpin, it is the least ideal pairing but can be scaled to fit the small number of peers that cannot establish a direct peering either through the "Local Address" or STUN discovered "Reflexive Address".
- (Future work) The relay node will likely be the point at which source ports are opened and reconciled against in peer listings.
- (Future work) Eliminating the need to terminate wireguard sessions to the relay node would reduce the attack surface and better adhere to zero trust networking principles. [Issue 169](https://github.com/nexodus-io/nexodus/issues/169)

### Health checking and proper probing (future work)

- Current probing is remedial at best. It needs to probe for a likely wireguard pairing and not just general connectivity even though we know the peer is running the agent and presume there is a wireguard port listening.
- Ideally we probe for a crypto pairing capability.
- Constant Health checking will be required to enable nodes to flip to different peerings as underlying topological changes occur. For example, if the STUN socket between two peers is no longer available, the two peer nodes should fall back to the relay node to enable continuity in the connection. Vice, versa, if a better peering candidacy returns, the nodes should flip back to direct peering either through the local address or reflexive address (Figure 1).

### Who determines peer candidates?

- Whether the controller determines peering matches or provides enough information about peers to the agent to determine proper candidacy is still tbd. Ideally, the controller is not in the process outside of peer listing updates allowing the agents to negotiate peering on the fly.
