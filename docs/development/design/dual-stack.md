# Dual Stack API Server and Driver Support

## Summary

Add dual stack support to Nexodus

## Proposal

This proposal is to support v6 addressing in both the api-server and wireguard driver. By default, every driver current and future, should get a v6 address along with a v4 address (example wg0). The initial work will encapsulate v6 over v4. IPv6 as a tunnel endpoint for encapuslated v4 transport can be future work.

### Schema Changes

The following schema changes are new as part of this implementation:

| Table        | New Field              | Function                                                |
|--------------|------------------------|---------------------------------------------------------|
| Device       | organization_prefix_v6 | v6 prefix constant `200:/64`                            |
| Device       | local_ip_v6            | v6 tunnel endpoint                                      |
| Device       | tunnel_ip_v6           | v6 address assigned to the driver's interface (ex. wg0) |
| Organization | ip_cidr_v6             | v6 prefix constant `200:/64`                            |

While `local_ip_v6` (v6 tunnel endpoint) is not used in the dual stack implementation, it makes sense to go ahead and add the table migration for a v4 over v6 future implementation.

### CLI Changes

The only CLI changes are some details in descriptions specifically calling out IPv4 only support to features such as `router --advertise-cidr` and `tunnel-ip` that are not implemented in the initial work. Ideally we don't add flag bloat here and accept v4/v6 in applicable fields rather than net new fields for v6. For example, `--advertise-cidr=172.17.20.0/24,2001:db8:abcd:0012::0/64`.

### IPv4 Only Support

Nexd needs to be able to accommodate hosts that do not support IPv6. Some container runtimes such as Docker have v6 disabled. There needs to be support checks at nexd initialization and flag the endpoint in the Nexodus receiver for peering logic to determine whether to add a dual or single stack to the driver's device interface and same for routes.

This should probably be transparent to the users. There probably isn't much value in giving a user the option to opt-in or out of v6. It does force future features to have v6 support which is likely a good practice. Policy addition would be an example of a feature that would require both v4 and v6 with the implementation if we don't give the user an option to opt out of dual stack and worth taking into consideration.

### Dual Stack Relay Node Support

Relay support for v6 is required just as it is for v4.

- Spoke node: If a node is in a symmetric NAT cone or been passed the `--relay-only` option, the joining node will receive a supernet prefix of the Organization's v6 CIDR of `200:/64` to the driver dev.
- Relay node: The `relay` needs to have v6 forwarding enabled on initialization (enable it if it is not enabled). Next an ip6tables rules are added to forward traffic in and out of the driver interface for relay only spokes.

### Default Org v6 Prefix

There is no CG-NAT equivalent for v6. Proposals have been drafted and submitted to the IETF to use `200:/8` with no traction. Since that is the closest thing to a de-facto we are using `200:/64` from that range for the initial Organization range. This is easily modified as it will be a constant and generally should not be hardcoded anywhere.

### Example Output

Example output from the Wireguard driver interface would look as follows:

```shell
$ ip address show wg0
1443: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 100.100.0.1/32 scope global wg0
       valid_lft forever preferred_lft forever
    inet6 200::1/64 scope global
       valid_lft forever preferred_lft forever
```

Example routing output from a meshed peer would have v6 host routes along with the v6 supernet corresponding to the wireguard peer to the relay node. For the small number of expected symmetric NAT nodes, they would only have the supernet route since they are incapable of NAT-T hole-punching.

```shell
$ ip route
...
100.100.0.2 dev wg0 scope link

$ ip -6 route
200::2 dev wg0 metric 1024 pref medium
200::3 dev wg0 metric 1024 pref medium
200::4 dev wg0 metric 1024 pref medium
200::5 dev wg0 metric 1024 pref medium
200::/64 dev wg0 proto kernel metric 256 pref medium
```

### Implementation Details

Specific details around the dual stack implementation are in the following issue.
> [Issue #637](https://github.com/nexodus-io/nexodus/issues/637)
