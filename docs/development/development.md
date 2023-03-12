# Nexodus Principles

## Guiding Principles

Our guiding principles help guide our decision-making. We use these principles when adding new features or making difficult decisions that require weighing different tradeoffs.

1. **Service First** -- We intend for Nexodus to run as a service, whether a public service or one run internally by an organization. To ensure that we get this right, the Nexodus development team must also run it as a service. We are doing this by following [open-source services](https://www.operate-first.cloud/community/open-source-services.html) principles: <http://try.nexodus.io/>. See more in [docs/deployment.md](docs/deployment.md).
2. **Simple UX Above Features** -- Networking technology is often made incredibly complex. One key value that the Nexodus service can provide is a simplified user experience for a much more challenging problem. When deciding how or if to add a feature, we value retaining the simplicity of the user experience over functionality.
3. **Secure by Design** -- Security at all levels is critical for this to be a viable service. No features are worth a regression in security. See [the next section](#nexodus-security) for more commentary on security in Nexodus.
4. **Optimized Connectivity** -- Forcing traffic through a central hub or other intermediary does not provide ideal network connectivity. Instead, we will implement various techniques to provide direct, mesh connectivity wherever possible, even in places where NAT or firewalls would typically prevent it.
5. **Platform Agnostic** - Nexodus will work independently from the infrastructure platform and support multiple operating systems (Linux, Mac, Windows). We value making the service easy to use with different infrastructure platforms (Kubernetes, for example), but we will avoid changes that tie the service to any particular platform.

## Nexodus Security

Security is critical for Nexodus to be a viable service. This document discusses some of the principles used when considering how to manage security for Nexodus.

1. **Secure Traffic** -- All traffic must be encrypted and the service should never have the ability to decrypt it. Even in the case that traffic needs to be relayed to provide connectivity, the relay must not be able to decrypt the traffic it is relaying[^1].
2. **Zero Trust Architecture** -- The service has been designed with the tenets of Zero Trust Networking in mind.
3. **Continuous monitoring and validation** -- Devices check in to the service regularly for peer updates. User logins are managed via OIDC which allows the use of short-lived access tokens that can be periodically refreshed. In effect, both Devices and Users check in regularly and their Peers or Tokens can be quickly revoked if needed.
4. **Device Authorization** -- Our device onboarding process associates devices with authenticated users.
5. **Least Privilege** -- Users are assigned only the privileges they require[^2].
6. **Microsegmentation** -- Users are broken into smaller segments using Zones and may only communicate within the same zone.

[^1]: See [Issue #169](https://github.com/nexodus-io/nexodus/issues/169). Our first forwarding implementation is by using a normal nexodus node on the network which decrypts, makes an IP routing decision, and sends it back out over another encrypted tunnel. This was based on convenience, but we know an alternative is required. In the meantime, security is maintained by having organizations operate their own relays instead of the service running them on their behalf.  
[^2]: See [Issue #142](https://github.com/nexodus-io/nexodus/issues/142) for tracking the use of oauth scopes to limit actions available to users.

### Reporting Vulnerabilities

Vulnerabilities may be reported privately via [GitHub](https://github.com/nexodus-io/nexodus/security/advisories). For more information about privately reporting security issues via GitHub, see the [GitHub Documentation](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability).
