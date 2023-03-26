# Proxy Mode for `nexd`

The normal mode of operation for `nexd` is to create a tunneled network interface on the device with an IP address within a Nexodus organization. Creating this network interface requires elevated privileges, so it is not usable in all environments.

Containers are a common example of an environment where `nexd` cannot be run in its normal mode. In these cases, `nexd` can be run in `proxy` mode. In proxy mode, `nexd` will not create a tunneled network interface, but will instead operate as a layer 4 proxy.

## Proxy Rules

Proxy rules must be specified as command line flags to `nexd` after specifying the `proxy` subcommand.

!!! warning "Beware of placement of flags to `nexd`"

    `nexd` has both general flags and subcommand-specific flags. When running `nexd` in proxy mode, the subcommand-specific flags must be specified after the subcommand. For example, here is an example of providing a general flag, as well as a `proxy` subcommand-specific flag:

    ```console
    nexd --request-ip 100.100.0.50 proxy --ingress $INGRESS_PROXY_RULE
    ```

### Ingress Proxy

Ingress proxy rules are specified with the `--ingress` flag. This flag can be specified multiple times to specify multiple ingress proxy rules. This is the format for an ingress proxy rule:

```console
--ingress protocol:port:destination_ip:destination_port
```

* `protocol` - must be `tcp`
* `port` - the port on the host that the proxy will listen on for connections made from a network able to access this device.
* `destination_ip` - the IP address of the destination within a Nexodus organization that the proxy will forward traffic to.
* `destination_port` - the port on the destination within a Nexodus organization that the proxy will forward traffic to.

Here is an example showing an ingress proxy rule:

```console
nexd proxy --ingress tcp:443:10.10.100.152:8443
```

```mermaid
flowchart TD
 linkStyle default interpolate basis
 device1[Remote device running nexd<br/><br/>IP: 100.100.0.1<br/><br/>Initiates connection to 100.100.0.2:443]-->|tunnel|network{Nexodus Network<br/><br/>100.100.0.0/16}
 network-->|tunnel|container[Container running nexd in proxy mode.<br/><br/>Nexodus IP: 100.100.0.2<br/>Local Network IP: 10.10.100.151<br/><br/>Accepts connections on 100.100.0.2:80 and forwards to 10.10.100.152:8443]

 subgraph Local Network - 10.10.100.0/24
 container-->|tcp|dest(Destination listening on port 8443<br/><br/>Local Network IP: 10.10.100.152)
 end
```

### Egress Proxy

Egress proxy rules are specified with the `--egress` flag. This flag can be specified multiple times to specify multiple egress proxy rules. This is the format for an egress proxy rule:

```console
--egress protocol:port:destination:destination_port
```

* `protocol` - must be `tcp`
* `port` - the port that `nexd` will accept connections to made to its IP address within the Nexodus organization this device is a member of.
* `destination` - the IP address or hostname of the destination on a network accessible to the device that the proxy will forward traffic to.
* `destination_port` - the port on the destination on a network accessible to the device that the proxy will forward traffic to.

Here is an example showing an egress proxy rule:

```console
nexd proxy --egress tcp:443:100.100.0.1:8443
```

```mermaid
flowchart TD
 linkStyle default interpolate basis
 network{Nexodus Network<br/><br/>100.100.0.0/16}-->|tunnel|device1[Remote device running nexd <br/><br/>Nexodus IP: 100.100.0.1<br/><br/>Running a service that listens on TCP port 8443]
 container[Container running nexd in proxy mode.<br/><br/>Nexodus IP: 100.100.0.2<br/>Local Network IP: 10.10.100.151<br/><br/>Accepts connections on 10.10.100.151:443 and forwards to 100.100.0.1:8443]-->|tunnel|network

 subgraph Local Network - 10.10.100.0/24
 dest(Source application connecting to port 443 on 10.10.100.151<br/><br/>Local Network IP: 10.10.100.152)-->|tcp|container
 end
```
