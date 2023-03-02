# Apex Security

Security is critical for Apex to be a viable service. This document discusses some of the principles used when considering how to manage security for Apex.

1. **Secure Traffic** -- All traffic must be encrypted and the service should never have the ability to decrypt it. Even in the case that traffic needs to be relayed to provide connectivity, the relay must not be able to decrypt the traffic it is relaying[^1].
2. **Zero Trust Architecture** -- The service has been designed with the tenets of Zero Trust Networking in mind.
3. **Continuous monitoring and validation** -- Devices check in to the service regularly for peer updates. User logins are managed via OIDC which allows the use of short-lived access tokens that can be periodically refreshed. In effect, both Devices and Users check in regularly and their Peers or Tokens can be quickly revoked if needed.
4. **Device Authorization** -- Our device on-boarding process associates devices with authenticated users.
5. **Least Privilege** -- Users are assigned only the privileges they require[^2].
6. **Microsegmentation** -- Users are broken into smaller segments using Zones and may only communicate within the same zone.

[^1]: See [Issue #169](https://github.com/nexodus-io/nexodus/issues/169). Our first forwarding implementation is by using a normal apex node on the network which decrypts, makes an IP routing decision, and sends it back out over another encrypted tunnel. This was based on convenience, but we know an alternative is required.
[^2]: See [Issue #142](https://github.com/nexodus-io/nexodus/issues/142) for tracking the use of oauth scopes to limit actions available to users.
