# Authorization

> [Issue #142](https://github.com/redhat-et/issues/142)

## Summary

We have 2 requirements to address in this design proposal:

1. We MUST prevent users from performing actions that they don't have permission to do
1. We MUST ensure that any API tokens obtain only the permissions they need (least privilege).

This will be addressed by implementing:

1. Some form of Access Control
1. OAUTH Scopes

These two features can share the same information since we're making a policy decision based on:

1. The scopes in the API token
1. A policy regarding the resource in question

## Proposal

### OAUTH Scopes

The following scopes will be used to restrict API token usage:

| Resource | Action | Scope |
|----------|--------|-------|
| organization | create | write:organizations |
| organization | delete | write:organizations |
| organization | update | write:organizations |
| organization | read   | read:organizations  |
| user         | delete | write:users         |
| user         | update | write:users         |
| user         | read   | read:users         |
| device       | create | write:devices       |
| device       | delete | write:devices       |
| device       | update | write:devices       |
| device       | read   | read:devices        |

### Access Control

We want to enforce the following policy:

1. Any user may create an organization, of which they become the owner.
1. Within an organization, an owner can issuing invitations to join; revoking membership; and off-boarding devices.
1. Within an organization, a user may onboard one (or more) of the devices they own.
1. Any action by any user is ALWAYS dependent on the API token scopes.

In order to enforce this model we need to be able to consult a policy to determine the users permissions based on API token and the resource owner.

It is also likely that at some point we're going to want to change the policy entirely, so attribute-based access control (ABAC) will give us the flexibility that we need. Having a policy language that is easy to write will also help with this as it may at some point be something that we expose to users.

Our apiserver already has access to the scopes in the token however we must also provide access to role membership and resource owner.

#### Resource Owners

The resource owner can usually be inferred by data we have on hand when processing API requests anyway.

For User, it is implicit.
For Device, it is the UserID.
For Organization, we will need to store an OwnerID.

#### Delegating Permissions

While not immediately required, we MAY wish to enable a resource owner to delegate some permissions. For example, an organization owner might wish to allow a subset of users permissions to manage permissions on their behalf.

Whatever solution we pick for authz should support this scenario.

### Open Policy Agent (OPA)

Our design is to use OPA, either as a library or a service within the Nexodus stack.

Pros:

- Can be run separately or embedded as a library
- Is a CNCF project
- Supports a wide range of policies types
- Policy language is easy to grok
- Has native support for validating JWTs as well as integrations with external data sources

Cons:

- None

### Implementation Details

We'll initially embed OPA into the apiserver using the Go SDK.

1. Add an `OwnerID` column to the `Organizations` table
1. Write policy, including test cases
1. Add calls via the Go SDK to check policy on every API route

#### Supporting Delegating Permissions

We can choose how best to do this in a future proposal.
As it stands there are a couple of options:

1. Exposing the organization policy to users directly, allowing an org owner to change the policy to their liking.
2. Allowing an organization owner to add "admins", and for us to render a policy for that organization that allows these admins the necessary access
3. Keep admin role information in our own database, and use the "overload input" or "push data" strategy to update OPA's data for policy decisions
4. Come up with a design that allows "Roles" to be added to the database

## Alternatives Considered

### Keycloak Authorization

[Keycloak Authorization Services](https://www.keycloak.org/docs/latest/authorization_services/#_service_overview)

Built-in to Keycloak and based on OAUTH2 and User Managed Access (UMA) standards.

Pros:

- We already have Keycloak in the stack

Cons:

- The UMA spec seems complicated. See: Permission Tickets.
- Overhead of one REST call per route for permissions check + a REST call to update the Resource APIs on resource creation.
- Ties us into Keycloak for Authz, meaning that we lose OIDC provider portability.

### Casbin

[Casbin](https://github.com/casbin/casbin#how-it-works)

A popular OSS library for authorization.

Pros:

- Can be embedded as a library
- Supports a wide range of authz policies

Cons:

- While actively maintained, the governance model is unclear.
- The policy language isn't as easy to grok as OPA.
