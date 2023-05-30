# Using a single CG-NAT IPv4 Address space by default

> [Issue #1011](https://github.com/nexodus-io/nexodus/issues/1011)

## Summary

Currently, each org gets a new IPAM address space for IP address allocation. This means the same IP address could be allocated to devices in different organizations. This would prevent a device in from being shared with other organizations.

Instead, organizations should default to using the shared CG-NAT address space which is 100.64.0.0/10, i.e. IP addresses from 100.64.0.0 to 100.127.255.255. This is about 4 Million addresses.

## Proposal

Why we should be using a single CG-NAT IPv4 Address space in Nexodus:

1. Giving devices a stable IP identity is paramount.  This identity should be preserved even if the device is moved from one organization to another organization.
2. In the future, it may be desirable to share individual devices with another user or organization.  If organizations use overlapping CIDRs and repeating addresses, this will not be possible.
3. Tailscale is using this GG-NAT range, and it seems like they are not having issues with running out yet.
4. If in the future we do start running out of address spaces and are we are willing to give up on #2, then we allocate additional CG-NAT address spaces.  Another option would be to use an IPv6 address space instead.
5. This also avoids needing a network administrator to pre-plan network CIDRs and how they need to be allocated to all the devices.
6. We can still allow an Organization to configure a different address space than the shared CG-NAT address space.  This would give control back to network administrators that want more control over the IP allocations.
7. Just because organizations share an address space does not mean the devices will be able to interconnect across organizations since WG public keys would not be shared between devices in different organizations.
8. All Nexodus devices are /32 routed at the current time.  In the future we may want to group devices into subnets to reduce the size of the routing tables that needs to be distributed, but even then we could take the subnet slices from the CG-NAT space.  Yes we should have to make the slices small to avoid wasting unallocated IP addresses.  Still this is not a current problem.

In conclusion, in the short term (since we are not going to be running out of addresses soon) we don't need an address space per organization.  Having a shared address space keeps our options open to implementing more interesting cross organization device sharing.  And we can still in the future use multiple address spaces if we start running out of addresses.

## Alternatives Considered

* Keep things as they are and don't share by default.  This would make sharing a device in an org, or moving a device from one org to another harder.

## References

* [Carrier-grade NAT](https://en.wikipedia.org/wiki/Carrier-grade_NAT)
