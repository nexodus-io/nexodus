# Documentation Home

![Nexodus](assets/wordmark.png#only-light)
![Nexodus](assets/wordmark_dark.png#only-dark)

Welcome to the Nexodus project documentation. A rendered version of these docs are available at <https://docs.nexodus.io>. You can also visit the project home page at <https://nexodus.io>. Feedback is welcome via [GitHub Issues](https://github.com/nexodus-io/nexodus/issues/new).

## Project Vision

This project aims to provide connectivity between nodes deployed across heterogeneous environments (Edge, Public, Private, and Hybrid Cloud) with different visibilities (nodes in a Cloud VPC, nodes behind NAT, etc.). This solution is not specific to any infrastructure or application platform but focuses on providing connectivity between nodes and the container or VM workloads running on those nodes. This service is complementary to platforms-specific networking, as it can expand connectivity to places the platform could not reach otherwise.

Some of the features and use cases that this project aims to support are:

- **Edge networking** - connectivity to any node, anywhere
- **Hybrid data center connectivity** - circumvents NAT challenges
- **IP mobility** - /32 host routing allows addresses to be advertised anywhere with convergence times only limited by a round-trip time to the Nexodus Service.
- **L4 Proxy** - TCP and UDP proxy mode to allow for connectivity to and from services running in non-privileged application environments (containers, for example).
- **Compliance** - Provide isolated connectivity among a set of nodes, even if they are running across multiple network administrative domains.
- **Configurable Authentication** - Nexodus uses OpenID Connect (OIDC) for authentication, allowing you to choose from a broad ecosystem of OIDC providers. OIDC gateways are available to provide interoperability with other authentication schemes (i.e., LDAP).

## Concepts

- **Users** – Nexodus uses OpenID Connect (OIDC) for authentication, allowing you to choose from a broad ecosystem of OIDC providers.
- **Organizations** – An isolated realm of connectivity. Multiple organizations allow for multi-tenant operations. This tenancy model is similar to GitHub or Quay.
- **Devices** – Endpoints that are connected to an organization and owned by a user.
- **Service** - The Service is the hosted portion that handles authentication, authorization, management of organizations, enrollment of devices, and coordination among devices to allow them to peer with each other.
- **Agent** - The Agent runs on each device. It communicates with the Nexodus Service and manages local network configuration.
