# Project Name

> [Issue #428](https://github.com/nexodus-io/nexodus/issues/428)

## Summary

Propose a new name for this project: Nexodus.

While we're at it, standardize naming across components.

## Proposal

The current name (Apex) is heavily used, including in a number of places in IT.
A more unique name for the project will make it more uniquely recognizable, as
well as more memorable.

* Proposed Project Name: **Nexodus**
* Proposed GitHub URL: <https://github.com/nexodus-io/nexodus>
* Proposed Project Domain: `nexodus.io`

 Currently we are using multiple names for different components across our documentation and code, such as:

* Apex Stack is referred to as Apex Service, Apex ApiServer, Apex Controller, Apex Control Plane, Apex SaaS
* Apex Agent is referred to as hub-router, relay, agent, discovery node

The following sections aim to clarify preferred naming.

### Binary Names

Note that there is already a `nex` binary, so we should not use that name.

```sh
$ dnf search nex
...
nex.x86_64 : A lexer generator for Go that is similar to Lex/Flex
```

* Proposed Binary Names
  * `apexd` to `nexd`
  * `apexctl` to `nexctl`
  * `apiserver` can remain. We only expect it to run inside a container and not a general client environment.

### Component Names

* Nexodus Service: This is an umbrella term to refer to the entire control plane stack that contains the following individual components (Named as is).
  * Nexodus ApiServer
  * Nexodus Frontend
  * Nexodus Database
  * Nexodus Ipam
  * Nexodus ApiProxy
  * Nexodus backend-cli
  * Nexodus backend-web
* Nexodus Agent: Agent running on each node, communicating with the Nexodus Service and managing local network configuration.
* Nexodus Relay: A node running the discovery and relay function.
  * At this time, the Nexodus Relay function is part of the Nexodus Agent. We refer to the Agent running in relay mode as the Nexodus Relay. In the future, the Relay function may be split out into its own separate binary.
* Nexodus Network: When referring to an overlay network managed by Nexodus, we refer to it as a "Nexodus network." At this time, a Nexodus organization maps to a single network, but we should not refer to the network as a "Nexodus organization," as that can be confusing. We should only use "Nexodus organization" when referring to the organizational construct that groups a subset of users and their devices together to communicate over a Nexodus network.

## Alternatives Considered

<https://github.com/nexodus/> is already taken.

We could use a variation of `apex` to provide a more unique identity.
<https://github.com/apex-mesh/> is available, for example. We already
own <https://apex-hosted.cloud>, so <https://github.com/apex-hosted/>
could be used as well.

An entirely different name could also be chosen, if anyone would like
to propose an alternative.
