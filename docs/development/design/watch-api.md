# Nexodus Device Event API Implementation Notes

## The Problem

In order to connect or disconnect devices from a Nexodus-managed organization, the list of all desired devices and security groups in the organization need to be replicated to all devices in the organization.  The `ListDevicesInOrganization` and `ListSecurityGroups` operations provides these lists and can be accessed via the following REST calls:

* `GET /api/organizations/{organization_id}/devices`
* `GET /api/organizations/{organization_id}/security_groups`

The `nexd` agent would need to periodically poll these endpoint to get the current desired state from the API server and reconcile the device configuration to add or remove Wireguard peers endpoints and firewall rules as necessary.

As the number of devices (N) in the organization increases, the number of periodic API requests to `ListDevicesInOrganization` increases linearly.  The size of the response list also increases linearly.  Put those two together, and you get the response bandwidth to the API server increasing exponentially (N requests/polling interval x N device elements in the response).  This will clearly lead to scaling challenges with large organizations.

This also has the problem that desired state changes recorded in the apiserver will not take effect immediately since the `nexd` agents are likely in the middle of their poll interval and won't poll the API server for a little while.

## The Solution: The Watch style API

Inspired by the Kubernetes List/Watch style APIs, the Nexodus now supports a `WatchEvents` operation that can be called to get an event stream of those resources.  Watchable resources have been given a `revision` field that can be used to efficiently make change tracking of devices possible.  When you call the `WatchEvents` operation, you give it an array of resource kinds that you want to watch.  Example:

* `POST /api/organizations/{organization_id}/events`
   with an example `application/json` request body

      [
        {
          "kind": "device",
        },
        {
          "kind": "security-groups",
          "gt_revision": 0,
          "at_tail": false
        }
      ] 

The API server will respond with a stream of changes. These changes itemize the outcome of operations (such as **change**, **delete**, and **update**) that occurred after the `revision` you specified in the `gt_revision` field in the request. The overall **watch** mechanism allows a client to fetch the current state and then subscribe to subsequent changes, without missing any events.

If a client **watch** is disconnected then that client can start a new **watch** from the last returned `revision`; the client could also perform a fresh watch without specifying a revision and begin again.

This has the effect of having better bandwidth scaling characteristics since only change events are sent to N devices when the desired state changes.  If the devices are not actively joining and leaving the organization, this can be very little bandwidth.  And even if they are joining and leaving at a constant rate, the bandwidth scales with N (not N x N).  In addition to the bandwidth savings, peer devices are pushed change events as soon as possible for processing which means they can converge on the desired state quicker than waiting for the next polling interval.

### Example

The `WatchEvents` operation HTTP response body (served as `application/json;stream=watch`) consists of a series of JSON documents.  When you don't specify the `gt_revision` argument, you will get all the resources in an organization like in a typical list request.  If the `at_tail` field in the request is false, or not set, then the list will be followed by a `tail` event.

1. List all the devices in a given organization.

   ```console
   POST /api/organizations/{organization_id}/events
   [
     {
       "kind": "device"
     }
   ]    
   ---
   200 OK
   Transfer-Encoding: chunked
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
   { "type": "tail" }   
   ```

   If a `WatchEvents` operation is disconnected then that client can start a new `WatchEvents` from
   the last returned `revision`

2. Starting from revision `10246`, use the following request to receive notifications of any API operations that affect devices. In this case we set the `"at_tail": true` so that we don't get teh tail marker again since the client has previously seen the tail marker already.

```console
   POST /api/organizations/{organization_id}/events
   [
     {
       "kind": "device",
       "at_tail": true,
       "gt_revision": 10246
     }
   ]    
   ---
   200 OK
   Transfer-Encoding: chunked
   Content-Type: application/json;stream=watch

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

### Client Library Access

The traditional non-watch API for listing devices in an organization is:

```go
devices, _, err := client.DevicesApi.
   ListDevicesInOrganization(context.Background(), orgID).
   Execute()
```

To use the `WatchEvents` operation version of the API, use:

```go
stream, _, err := client.OrganizationsApi.
   WatchEvents(context.Background(), orgID).
   Watches([]public.ModelsWatch{
      {
          Kind: "device",
          GtRevision: 0,
          AtTail: false,
      }
   }).
   WatchEventStream()

defer watch.Close()

// get all the events..
for {
    event, err := stream.Receive()    
}

```

To simplify working against the event-based `WatchEvents` operation we have implemented a Kubernetes-style Informer API.

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

To get different informers sharing a single `WatchEvents` API call, you need to have them use a `context.Context`
created using the `client.OrganizationsApi.WatchEvents(ctx, orgID).NewSharedInformerContext()` call.  Example:

```go
informerCtx := client.OrganizationsApi.
   WatchEvents(ctx, orgID).
   NewSharedInformerContext()

devicceInformer := client.DevicesApi.
   ListDevicesInOrganization(informerCtx, orgID).
   Informer()

securityGroupsInformer := client.SecurityGroupApi.
   ListSecurityGroups(informerCtx, orgID).
   Informer()

for {
    select {
        case <- ctx.Done():
            return
        case <- devicceInformer.Changed():
            devices, _, err := devicceInformer.Execute()
            ... process the devices ....
        case <- securityGroupsInformer.Changed():
            securityGroups, _, err := securityGroupsInformer.Execute()
            ... process the security securityGroups ....
    }
}
```

### Apiserver Implementation of `WatchEvents`

The HTTP request handler servicing the `WatchEvents` will for each requested watch:

1. Create a **SignalBus** (described later in this doc) subscription to `/devices/org={orgId}`
2. Select matching devices from the DB.  It will force device results to be ordered by `revision` so that the last selected result has the highest revision.
3. Each result will be sent to the client as an independent JSON document.  This will continue until it gets an empty result set from the database.
4. At that point, it will send the tail event to the client if `"at_tail": false`
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
    Signal() <-chan struct{}
    // close out the subscription.
    Close()
}
```

Since this interface does not hold events and even coalesces multiple Notify calls, it results in always being non-blocking to the callers of Notify and always having a bounded amount of memory that it uses.

We have also implmented a distributed version of the SignalBus interface using the [LISTEN/NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html) postgresql SQL statements.  This allows the **apiserver** to still be able to notify watch requests in other processes and thus safely scale the number of apiserver replicas past 1.

#### Scaling up large Organizations

As you you start watching for change events in large organizations with many devices, it becomes harder to scale the apiserver and the backing SQL database.  This is because when a device record is changed, every connected device will have an active connection requesting those event changes and so the API server will wake up each idle connection in the long poll, and have it poll the database to get those changed device records.  Let us say you have an organization with 10,000 connected devices.  When one device is changed, that will cause 10,000 SQL queries to be executed to find out what has changed.

To reduce the load on the database, we have implemented a per organization device tail cache. When a client now requests an event stream of device changes, it will execute SQL queries against the database until it reaches the tail of the change stream (this occurs when the DB query is an empty result set).  At this point now it will look for the device change record a ring buffer that is shared with all API calls looking for device changes for the same organization.  When one of the clients reaches the tail of the ring buffer, it fill the ring  with more device changes by polling the database once again. Note that only 1 is allowed to fill at a time.  If the other clients can't keep up with the fill speed of the ring buffer it, will fall back to polling the database of change data and only go back to using the ring buffer once it reaches the tail of the change stream again.  This avoids stalling event stream consumers with slow event stream consumers and caps the amount of memory the tail cache can use to be proportional to the ring buffer size.