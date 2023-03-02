# Authentication

The Nexodus Stack uses OpenID Connect for authentication.
It allows whomever deploys the stack to chose any OpenID connect provider they wish in order to provide user authentication.
It also enables Nexodus to focus on its core, and to defer authentication to another service.

## Web Frontend Authentication

The web frontend authentication follows the Backend For Frontend (BFF) architecure. The nexodus stack has 2 components:

- go-oidc-agent
- apiserver

The [go-oidc-agent](https://github.com/redhat-et/go-oidc-agent) service is a dedicated backend for the web frontend that provides authentication services, and proxies API requests to the apiserver.
This not only helps simplify deployment, but also reduces risk of compromise of access tokens/refresh token compromise by keeping them out of the browser.

For more information on this flow see:

- <https://github.com/redhat-et/go-oidc-agent>
- <https://auth0.com/blog/backend-for-frontend-pattern-with-auth0-and-dotnet/>
- <https://curity.io/resources/learn/the-token-handler-pattern/>

The apiserver expects to see JWTs in the `Authorization` header, and will validate the JWT signature against the OpenID Providers JWKs.

## CLI Frontend Authentication

The cli frontend authentication follows the Backend For Frontend (BFF) architecure also. The nexodus stack has 2 components:

- go-oidc-agent
- apiserver

When `go-oidc-agent` is used in Device Flow mode, it simply sends:

1. The Device Authorization Endpoint of the Authenication Server
1. The Client ID to use

The Nexodus CLI is then responsible for acquiring and storing tokens.
In this case, the `go-oidc-agent` is there to simplfy deployment only - so no endpoints or client-ids need to be included in the client binary, or injected through config file or flags.
