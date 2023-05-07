# Nexodus Device Watch API Implementation Notes

## The Problem

In order to connect or disconnect devices from a Nexodus-managed organization, the list of all desired devices in the organization need to be replicated to all devices in the organization.  The `ListDevicesInOrganization` operation provides this list and can be accessed via the following REST call:

`GET /api/organizations/{organization_id}/devices`

The `nexd` agent would need to periodically poll this endpoint to get the current desired state from the API server and reconcile the device configuration to add or remove Wireguard peers endpoints as necessary.

As the number of devices (N) in the organization increases, the number of periodic API requests to `ListDevicesInOrganization` increases linearly.  The size of the response list also increases linearly.  Put those two together, and you get the response bandwidth to the API server increasing exponentially (N requests/polling interval x N device elements in the response).  This will clearly lead to scaling challenges with large organizations.

This also has the problem that desired state changes recorded in the apiserver will not take effect immediately since the `nexd` agents are likely in the middle of their poll interval and won't poll the API server for a little while.

## The Solution: The Watch style API

Inspired by the Kubernetes List/Watch style APIs, the Nexodus `ListDevicesInOrganization` operation can be called with the `watch=true` query parameter.  Furthermore, devices have been given a `revision` field that can be used to efficiently make change tracking of devices possible.

When you call the `ListDevicesInOrganization` operation with the `watch=true` query parameter, the API server responds with a stream of changes. These changes itemize the outcome of operations (such as **change**, **delete**, and **update**) that occurred after the `revision` you specified as a query parameter to the request. The overall **watch** mechanism allows a client to fetch the current state and then subscribe to subsequent changes, without missing any events.

If a client **watch** is disconnected then that client can start a new **watch** from the last returned `revision`; the client could also perform a fresh watch without specifying a revision and begin again.

This has the effect of having better bandwidth scaling characteristics since only change events are sent to N devices when the desired state changes.  If the devices are not actively joining and leaving the organization, this can be very little bandwidth.  And even if they are joining and leaving at a constant rate, the bandwidth scales with N (not N x N).  In addition to the bandwidth savings, peer devices are pushed change events as soon as possible for processing which means they can converge on the desired state quicker than waiting for the next polling interval.

### Example

When you use the `watch=true` query parameter, the HTTP response body (served as `application/json;stream=watch`) consists of a series of JSON documents.  When you don't specify the `revision` argument, you will get all the devices in an organization like in a typical list request followed by a `bookmark` event.

1. List all of the devices in a given organization.

   ```console
   GET /api/organizations/{organization_id}/devices?watch=true
   ---
   200 OK
   Content-Type: application/json

   {
     "type": "change",
     "value":  {
       "user_id": "3833f886-fc40-44df-b4e3-644fa287ed2d",
       "organization_id": "f13fe84d-9d6e-4dad-8464-bb30e9ef0194",
       "public_key": "...",
       "local_ip": "172.17.0.4:58664",
       "tunnel_ip": "",
       "child_prefix": null,
       "relay": false,
       "discovery": false,
       "reflexive_ip4": "47.196.141.166",
       "endpoint_local_address_ip4": "172.17.0.4",
       "symmetric_nat": true,
       "hostname": "device2",
       "revision": 10246
     }
   }
   { "type": "bookmark" }   
   ```

If a client **watch** operation is disconnected then that client can start a new **watch** from
the last returned `revision`;
2. Starting from revision 10245, use the following request to receive notifications of any API operations that affect devices.

   ```console
   GET /api/organizations/{organization_id}/devices?watch=true&revision=10245
   ---
   200 OK
   Transfer-Encoding: chunked
   Content-Type: application/json;stream=watch

   { "type": "bookmark" }
   ...
   {
     "type": "delete",
     "value":  {
       "user_id": "3833f886-fc40-44df-b4e3-644fa287ed2d",
       "organization_id": "f13fe84d-9d6e-4dad-8464-bb30e9ef0194",
       "public_key": "...",
       "local_ip": "172.17.0.4:58664",
       "tunnel_ip": "",
       "child_prefix": null,
       "relay": false,
       "discovery": false,
       "reflexive_ip4": "47.196.141.166",
       "endpoint_local_address_ip4": "172.17.0.4",
       "symmetric_nat": true,
       "hostname": "device2",
       "revision": 10247
     }
   }      
   ...
   ```

In the above example, the client gets a bookmark event immediately indicating no changes had occurred since revision 10245, but then a little while later a delete event is delivered.

### Client Library Access

The traditional non-watch API for listing devices in an organization is:

```go
devices, _, err := client.DevicesApi.
   ListDevicesInOrganization(context.Background(), orgID).
   Execute()
```

To use the `watch=true` version of the API, use:

```go
watch, _, err := client.DevicesApi.
   ListDevicesInOrganization(context.Background(), orgID).
   Watch()

defer watch.Close()

// get all the events..
for {
    kind, device, err := watch.Receive()    
}

```

To simplify working against the event-based *watch* operations we have implemented a Kubernetes-style Informer API.

An Informer, in essence, is a local cached copy of the resources that clients are interested in. The local cache will be refreshed via the watch operation. But the use of the watch API occurs asynchronously and is hidden from the clients.

To use the Informer API

```go
informer := client.DevicesApi.
   ListDevicesInOrganization(context.Background(), orgID).
   Informer()

devices, _, err := informer.Execute()
```

Notice that the result of `informer.Execute()` is the same as the traditional list-style API.  Additionally, an informer provides a channel via `informer.Changed()` that you can use to wait for when you can call `informer.Execute()` to get a changed device list.

```go
informer := client.DevicesApi.
   ListDevicesInOrganization(ctx, orgID).
   Informer()

for {
    select {
        case <- ctx.Done():
            return
        case <- informer.Changed():
            devices, _, err := informer.Execute()
            ... process the devices ....
    }
}
```

### Apiserver Implementation of `watch=true`

The HTTP request handler servicing the `ListDevicesInOrganization` will:

1. Create a **SignalBus** (described later in this doc) subscription to `/devices/org={orgId}`
2. Select matching devices from the DB.  It will force device results to be ordered by `revision` so that the last selected result has the highest revision.
3. Each result will be sent to the client as an independent JSON document.  This will continue until it gets an empty result set from the database.
4. At that point, it will send the bookmark event the the client
5. Parks the go routine waiting for the http request to be terminated or the **SignalBus** subscription to be notified.
6. It loops back and selects matching devices from the DB since that last seen revision.
7. Sends each selected device to the client,
8. loops back to #5

All API operations that change the desired device state singal the **SignalBus** on the `/devices/org={orgId}` when those devices are changed.

### What is a SignalBus

A **SignalBus** is a really simple interface that you can use to notify other go routines that a named thing has been signaled.

```go
type SignalBus interface {
    // Notify will notify all the subscriptions created for the given named signal.
    Notify(name string)
    // NotifyAll will notify all the subscriptions
    NotifyAll()
    // Subscribe creates a subscription the named signal
    Subscribe(name string) *Subscription
}

type Subscription interface { 
    // You can wait on this channel to know when it's been signaled
    Signal() <-chan bool
    // close out the subscription.
    Close()
}
```

Since this interface does not hold events and even coalesces multiple Notify calls, it results in always being non-blocking to the callers of Notify and always having a bounded amount of memory that it uses.

We have also implmented a distributed version of the SignalBus interface using the [LISTEN/NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html) postgresql SQL statements.  This allows the **apiserver** to still be able to notify watch requests in other processes and thus safely scale the number of apiserver replicas past 1.