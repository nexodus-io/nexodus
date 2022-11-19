Authentication
==============

The Apex Stack uses OpenID Connect for authentication.
It allows whomever deploys the stack to chose any OpenID connect provider they wish in order to provide user authentication.
It also enables Apex to focus on its core, and to defer authentication to another service.

## Web Frontend Authentication

The web frontend authentication follows the Backend For Frontend (BFF) architecure. The apex stack has 2 components:

- backend-web
- apiserver

The backend-web service is a dedicated backend for the web frontend that provides authentication services, and proxies API requests to the apiserver.

The frontend runs at `apex.dev`
The backend-web runs at `api.apex.dev`
The apiserver isn't publicly exposed

The frontend will redirect the user to `api.apex.dev/login`.
This route will in-turn, redirect the user to the configured OpenID Connect provider for authentication.
The authentication callback goes to `api.apex.dev/callback` where the `access_token`, `refresh_token` etc.. are stored in an encrypted cookie.

For more information on this flow see:
- https://auth0.com/blog/backend-for-frontend-pattern-with-auth0-and-dotnet/
- https://curity.io/resources/learn/the-token-handler-pattern/

Before API requests are proxied to the `apiserver`, the `backend-web` will check validity and acquire a new token (using the `refresh_token`) if required.

The apiserver uses expects to see JWTs in the `Authorization` header, and will validate the JWT signature against the OpenID Providers JWKs.
