# Nexodus Stack

## Summary

This proposal documents the decisions made around the Nexodus software stack.

## Proposal

### Kubernetes Only

#### Background

Early on, we used Docker Compose for orchestration.
As the stack and complexity grew, we decided to exclusively support Kubernetes, requiring local development to use KIND.

To support use of KIND for local development, and OpenShift for production deployment we settled on using Kustomize.

#### Decisions

- We only support deployment on K8s
- Kustomize is our deployment configuration tool
- All manifests in `deploy/base` MUST be useable in any K8s distribution.
- We allow operators to be used, provided they may also be installed on any K8s distribution.

### Microservices First

#### Background

An early agreed architectural principal of the Nexodus stack was to embrace microservices. This would allow us to iterate on components seperately, give us maximum re-use if the project failed, and most importantly, allow us to scale each component seperately.

This resulted in the following decisions:

1. IPAM would be a separate service, consumed over gRPC, since it already has an [upstream project](https://github.com/metal-stack/go-ipam).
1. Authentication would use Open ID Connect (OIDC), provided by default in our stack by Keycloak.
1. The `go-oidc-agent` proxy would be deployed as a binary, in-front of existing services vs. embedding that logic as library code in the apiserver.
1. We would use [gorm](https://gorm.io) as an ORM to give us choice to switch out the DB implementation if required.
1. We would use PostgreSQL as our database of choice, and deploy it via the Crunchy Data Postgres Operator.

However, one could argue that these decisions are in-fact related to how we consume dependencies OR how we deploy the stack, and less about microservices.

The current apiserver is a monolith backed by a relational database.

#### Decisions

- We value modularity, but we will compromise if being non-modular is simpler or offers a better developer or user experience.

## Alternatives Considered

If Microservices are truly important to us, we should reconsider the design of the apiserver. For example, we'd break this into:

- Authentication service: authenticates users
- User profile service: update/retrieve information for users
- Organization service: CRUD for organizations
- Device service: CRUD for devices
- Device onboarding service: Onboards/offboards a device into an organization
- Peering service: handles peer updates for devices

## References

*Leave links to helpful references related to this proposal.*
