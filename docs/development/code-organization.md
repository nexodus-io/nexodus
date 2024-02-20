# Code Organization

This document provides an overview of the code organization of Nexodus.

## Overview

### nexd - The Nexodus Agent

The entrypoint for `nexd` is in `cmd/nexd/main.go`. There are some additional files under `cmd/nexd`,
but the bulk of the agent code is found under `internal/nexodus/`.

### nexctl - The Nexodus CLI

The entrypoint for `nexctl` is in `cmd/nexctl/main.go`. The majority of `nexctl`'s code is found under
`cmd/nexctl`, as well.

If you want to add a new subcommand under `nexctl nexd ...`, the corresponding code on the `nexd` side
is found in the files following this pattern: `internal/nexodus/ctl*.go`.

### The Nexodus Web UI

All of the code for the Nexodus web UI is found under `ui/`.

### The Nexodus API Server

The entrypoint for `apiserver` is in `cmd/main.go`. Most of the code for the API server is found
under `internal/`.

* `internal/routers` - the code for mapping HTTP requests to the code that will handle them.
* `internal/handlers` - the code that handles the HTTP requests as mapped from the request router.
* `internal/database` - the code for managing the database schema and migrations.
* `internal/models` - the data structures that are used to interact with the database.

### Tests

The integration tests are found under `integration-tests/`. Unit tests are found throughout the code base,
typically with `_test.go` suffixes on the file names.

## Common Tasks

This section contains some pointers to where to start looking when making common changes to Nexodus.

### Changing the Database Schema

The current database schema is reflected in the data structures found in `internal/models/`. Changes to
the database schema are written as migrations, which are found under `internal/database/`. Migrations
allow changes to the database schema to be performed as part of an upgrade and are executed by the API
server when it starts up and recognizes that its database is now out of date.

1. Write a database migration in `internal/database/`.
2. Add or update models in `internal/models/`.

### API Changes

The current API for the production instance of Nexodus can be found at <https://api.try.nexodus.io/openapi/index.html>.
This documentation is automatically generated from the code base.

The handlers for each HTTP method for each resource in the API is defined in `internal/routers/routers.go`.
For example, these lines define the methods for devices, or `/api/devices`:

```go
        // Devices
        apiGroup.GET("/devices", api.ListDevices)
        apiGroup.GET("/devices/:id", api.GetDevice)
        apiGroup.PATCH("/devices/:id", api.UpdateDevice)
        apiGroup.POST("/devices", api.CreateDevice)
        apiGroup.DELETE("/devices/:id", api.DeleteDevice)
```

The handlers themselves are defined under `internal/handlers`. Adding or changing API behavior will be
done there.