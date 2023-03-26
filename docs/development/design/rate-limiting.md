``# Used Envoy for Rate Limiting

[Issue #600](https://github.com/nexodus-io/nexodus/issues/600)

## Summary

To protect the apiserver service from a misbehaving tenant, the REST APIs should be rate limited.
This will allow us to better reason about how to scale the service to a large number of tenants as there will be limit to the amount of apiserver resources each tenant can consume.

This also translates to a limit of number of devices that a tenant can enroll before he starts to hit rate limits.
There should be a way to increase the limits per tenant.

## Proposal

Use an [Envoy](https://www.envoyproxy.io/) to replace the current use of Caddy as an api proxy and [Limitador](https://github.com/Kuadrant/limitador) to enforce the rate limiting policies.

Using an Envoy proxy has the following benefits:

* Fast proxy used by many projects and supported by many organizations
* We could possibly move more functionality to it in the future (like oauth processing)
* Some K8s platforms are moving to envoy as the Ingress gateway so in the future we may be able to move this functionality into the ingress gateway (avoiding a proxy hop).
* Aligned with service mesh deployment models.

The login handlers for the frontend will need to set the `AccessToken` http only Cookie.  
API requests from devices will pass the AccessToken as a bearer token in the `Authentication` header.  
This will allow the Envoy proxy to have easy access to the AccessToken for requests being sent to the apiserver.  
The JWT AccessToken will then be validated in Envoy and it's claims passed to limitador to enforce per-user rate limits.  
Envoy can then send 429 responses for any requests that have been rate limited.  
Since a valid AccessToken is needed, we will be able to identify the tenant generating the source of the traffic even if client trying to create multiple sessions or changing source IPs.

The user's JWT `sub` claim will be used to identify a tenant.  
Requests for resources that do not need AccessToken (for example: html and js resources or requests to authenticate against the auth server) should be rate limited using the source ip address.

### Future Option: Run envoy as sidecar

Pros:

* More secure since the proxy and apiserver communicate on the loopback interface reducing the possibility of packet snooping.
* More efficient since the proxy and apiserver are guaranteed to run on the same worker node.
* Easier to reason about scaling with traffic load since both get scaled up as unit.

Cons:

* You can't use telepresence to debug pods that use sidecars in this way

## Alternatives Considered

Live without rate limiting.
If you have a custom Nexodus service deployment, and don't share it with multiple tenants, you may not need rate limiting.

## References

* [Envoy](https://www.envoyproxy.io/)
* [Limitador](https://github.com/Kuadrant/limitador)
