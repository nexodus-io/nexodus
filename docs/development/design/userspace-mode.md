# Userspace Mode L4 Proxy

> [Issue #187](https://github.com/nexodus-io/nexodus/issues/187)

## Summary

The traditional operation of `nexd` requires elevated privileges to create a network interface. This prevents usage in certain environments where providing these privileges is not possible. The primary example is a developer using containers. A development may have application tunneling needs to reach a service in some remote location, but is unable to run with elevated privileges. Nexodus should provide a mode that works for this use case.

## Proposal

The Go library from the Wireguard project, [wireguard-go](https://github.com/WireGuard/wireguard-go/), includes support for a Wireguard tunneling mode that operates entirely in userspace without creating a new network device in the operating system. Since there is no network device, the host does not get an IP address on a Wireguard network. Instead, an application using `wireguard-go` can initiate connections over Wireguard.

To support a wider set of use cases in Nexodus, the proposal is to allow `nexd` to run in a new userspace-only mode where it can act as a proxy between locally accessible applications and other devices accessible via the Wireguard network managed by Nexodus. This turns `nexd` into a layer 4 (transport) interface to a Nexodus network instead of layer 3 (IP).

### CLI Subcommands

To allow `nexd` to be run in a different mode, `nexd`'s command line interface shold be modified to make use of subcommands. This will allow a different set of flags to be defined for each operating mode. The first step is to move all existing flags under a new subcommand, `agent`.

```sh
sudo nexd agent https://try.nexodus.local
```

While the implementation is not considered in scope of this proposal, it should be considered a natural next step to change the `--relay-node` and `--discovery-node` flags into subcommands. For example:

```sh
sudo nexd relay https://try.nexodus.local
```

or

```sh
sudo nexd discovery https://try.nexodus.local
```

Once the current mode has been moved under an `agent` subcommand, a new `proxy` command can be added for this new userspace-only mode of interfacing with a Nexodus network.

## Fewer Flags for Proxy Mode

Only a subset of the flags used in `agent` mode apply to `proxy` mode. The following flags will not be present under the `proxy` mode:

```text
   --child-prefix value [ --child-prefix value ]  Request a CIDR range of addresses that will be advertised from this node (optional) [$NEXD_REQUESTED_CHILD_PREFIX]
   --relay-node                                   Set if this node is to be the relay node for a hub and spoke scenarios (default: false) [$NEXD_RELAY_NODE]
   --discovery-node                               Set if this node is to be the discovery node for NAT traversal in an organization (default: false) [$NEXD_DISCOVERY_NODE]
   --relay-only                                   Set if this node is unable to NAT hole punch in a hub zone (Nexodus will set this automatically if symmetric NAT is detected) (default: false) [$NEXD_RELAY_ONLY]
```

### Proxy Configuration

As a first step, `nexd` will take proxy configuration via two configuration flags: `--ingress` and `--egress`.

`--ingress` is used to proxy for connections coming from the Nexodus network to a server accessible by `nexd` via a local network.

`--egress` is used to proxy for connectoins coming from a network local to `nexd` to a server reachable over the Nexodus network.

In both cases, the format for the value is `[protocol]:[port]:[desination_ip]:[destination_port]`. `protocol` can be `tcp` or `udp` to start.

#### Example 1 - Expose a web service to a Nexodus network

Proxy connections coming from the Nexodus network on port 443 to a service running on localhost port 8080.

```sh
sudo nexodus proxy --ingress tcp:443:127.0.0.1:8080
```

#### Example 2 - Allow connections to a remote Postgres Database

Proxy connections made to `nexd` on port 5432 to a Postgres database running on a remote host on the Nexodus network.

```sh
sudo nexodus proxy --egress tcp:5432:100.100.0.2:5432
```

### Runtime Proxy Configuration

The `--ingress` and `--engress` flags will be the first configuration methods implemented. A future enhancement would be to allow configuration at runtime using `nexctl`. For example, to remove a ingress proxy rule:

```sh
sudo nexctl nexd proxy remove ingress tcp:443:127.0.0.1:8080
```

or to add a new egress proxy rule:

```sh
sudo nexctl nexd proxy add egress tcp:5432:100.100.0.2:5432
```

## Alternatives Considered

### New binary

Instead of adding subcommands to `nexd`, this functionality could reside in a new binary. In either case, it could still share most of the relevant code with `nexd`. This proposal follows the approach of minimizing the number of binaries used on devices: just `nexd` and `nexctl`.

### Modify the relay to proxy L4 connections

Another approach to this use case would be to require clients to connect to a `nexd` relay that can accept connections not yet secured by Wireguard and allow the relay to act as the proxy into and out of the Nexodus network. This doesn't fully solve the developer use case since running a `relay` requires elevated privileges. The alternative is requiring coordination with an ops team to ensure relays are available in an appropriate secure location for the applications that need this connectivity mode.

### SOCKS5 or L7 Proxy

There are other proxy methods that could be implemented instead of what is proposed here. For example, we could take inspiration from the approach taken by Tailscale's [userspace networking mode](https://tailscale.com/kb/1112/userspace-networking/) and work as a SOCKS5 proxy or an L7 (http, for example) proxy.

### Integration with an Existing Proxy

It could also make sense to try to integrate this functionality with an existing Proxy application instead of reimplementing the proxy functionality ourselves. For example, adding the functionality of `nexd proxy` as an extension to Envoy could be powerful. The feasibility of this hasn't been evaluated.

### Another programming language

`nexd` is normally not in the data path for Nexodus at all. `nexd proxy` puts it in the data path, so performance becomes more critical. Performance testing will determine whether this approach is viable for the long term or whether another implementation is warranted. Some interest has been expressed in building the data path piece in Rust. There is a [Wireguard client for Rust](https://github.com/cloudflare/boringtun), though it does not have the same userspace-only functionality provided by the Go library. It could be possible in combination with [smoltcp](https://github.com/smoltcp-rs/smoltcp) or something similar. At this time, no such implementation is proposed.
