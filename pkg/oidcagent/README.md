go-oidc-agent
=============

`go-oidc-agent` is a small binary designed to act as a Backend-For-Frontend, handling the OIDC authentication on behalf of the frontend app.

It is heavily influenced by [oauth-agent-node-express](https://github.com/curityio/oauth-agent-node-express) with the following notable differences:

1. It's written in Go
1. The session storage (used for tokens) is swappable thanks to `go-session/session` so it can use encrypted cookies, memcached etc...
1. It acts as a proxy for request from the frontend the configured backend API, adding the necessary authentication credentials.

There are also some omissions, which will need addressing before this can be used in production.

1. CORS

# Example Deployment

![design](./docs/go-oidc-agent-deployment.png)
