# Documentation

- [Documentation](#documentation)
  - [Concepts](#concepts)
  - [Guides](#guides)

## Concepts

- **Users** – Apex uses OpenID Connect (OIDC) for authentication, allowing you to choose from a broad ecosystem of OIDC providers.
- **Organizations** – An isolated realm of connectivity. Multiple organizations allow for multi-tenant operations. This tenancy model is similar to GitHub or Quay.
- **Devices** – Endpoints that are connected to an organization and owned by a user.
- **Service** - The Service is the hosted portion that handles authentication, authorization, management of organizations, enrollment of devices, and coordination among devices to allow them to peer with each other.
- **Agent** - The Agent runs on each device. It communicates with the Nexodus Service and manages local network configuration.

## Guides

- [Deploying the Nexodus Service](nexodus-service.md)
- [Nexodus User Guide](user-guide.md)
- [Nexodus Development](development.md)
