# ![Apex](assets/logo.svg)

> **Warning**: This project is currently experimental.

[![build](https://github.com/redhat-et/apex/actions/workflows/build.yml/badge.svg)](https://github.com/redhat-et/apex/actions/workflows/build.yml)

This project demonstrates an approach for building an IP connectivity-as-a-service solution that provides isolated zone-based connectivity using Wireguard for tunneling.

## Project Vision

This project aims to provide connectivity between nodes deployed across heterogeneous environments (Edge, Public, Private, and Hybrid Cloud) with different visibilities (nodes in a Cloud VPC, nodes behind NAT, etc.). This solution is not specific to any infrastructure or application platform but focuses on providing IP connectivity between nodes and the container or VM workloads running on those nodes. This service is complementary to platforms-specific networking, as it can expand connectivity to places the platform could not reach otherwise.

Some of the features and use cases that this project aims to support are:

- **IoT networking** - connectivity to any node, anywhere
- **Hybrid data center connectivity** - circumvents NAT challenges
- **IP mobility** - /32 host routing allows addresses to be advertised anywhere with convergence times only limited by a round-trip time to a controller.
- **Compliance** - Provide isolated connectivity among a set of nodes, even if they are running across multiple network administrative domains.
- **Configurable Authentication** - Apex uses OpenID Connect (OIDC) for authentication, allowing you to choose from a broad ecosystem of OIDC providers. OIDC gateways are available to provide interoperability with other authentication schemes (i.e., LDAP).

## Guiding Principles

Our guiding principles help guide our decision-making. We use these principles when adding new features or making difficult decisions that require weighing different tradeoffs.

1. **Service First** -- We intend for Apex to run as a service, whether a public service or one run internally by an organization. To ensure that we get this right, the Apex development team must also run it as a service. We are doing this on top of [operate-first](https://www.operate-first.cloud/) community infrastructure and by following [open-source services](https://www.operate-first.cloud/community/open-source-services.html) principles: <http://apex.apps.smaug.na.operate-first.cloud/>. See more in [docs/deployment.md](docs/deployment.md).
2. **Simple UX Above Features** -- Networking technology is often made incredibly complex. One key value that the Apex service can provide is a simplified user experience for a much more challenging problem. When deciding how or if to add a feature, we value retaining the simplicity of the user experience over functionality.
3. **Secure by Design** -- Security at all levels is critical for this to be a viable service. No features are worth a regression in security. There's enough to say about security that we've created a dedicated document for it: [docs/security.md](docs/security.md).
4. **Optimized Connectivity** -- Forcing traffic through a central hub or other intermediary does not provide ideal network connectivity. Instead, we will implement various techniques to provide direct, mesh connectivity wherever possible, even in places where NAT or firewalls would typically prevent it.
5. **Platform Agnostic** - Apex will work independently from the infrastructure platform and support multiple operating systems (Linux, Mac, Windows). We value making the service easy to use with different infrastructure platforms (Kubernetes, for example), but we will avoid changes that tie the service to any particular platform.

## Documentation

More detailed documentation covering how to use Apex for different scenarios is in the [project docs](docs/README.md).