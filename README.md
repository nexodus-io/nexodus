# ![Apex](assets/logo.svg)

[![build](https://github.com/redhat-et/apex/actions/workflows/build.yml/badge.svg)](https://github.com/redhat-et/apex/actions/workflows/build.yml)

> *Roads? Where we're going, we don't need roads - Dr. Emmett Brown*

This project demonstrates an approach for building an IP connectivity-as-a-service solution that provides isolated zone-based connectivity using Wireguard for tunneling.

## Project Vision

This project aims to provide connectivity between nodes deployed across heterogeneous environments (Edge, Public, Private, and Hybrid Cloud) with different visibilities (nodes in a Cloud VPC, nodes behind NAT, etc.). This solution is not specific to any infrastructure or application platform but focuses on providing IP connectivity between nodes and the container or VM workloads running on those nodes. This service is complementary to platforms-specific networking, as it can be used to expand connectivity to places that the platform could not reach otherwise.

Some of the features and use cases that this project aims to support are:

- **IoT networking** - connectivity to any node, anywhere
- **Hybrid data center connectivity** - circumvents NAT challenges
- **IP mobility** - /32 host routing allows addresses to be advertised anywhere with convergence times only limited by a round-trip time to a controller.
- **Compliance** - Provide isolated connectivity among a set of nodes, even if they are running across multiple network administrative domains.
- **Platform Agnostic** - Work independently from the infrastructure platform and support multiple operating systems (Linux, Mac, Windows).
- **SaaS** - Built with a service-first mindset and provides enterprise auth out of the box

## Documentation

More detailed documentation covering how to use Apex for different scenarios can be found in the [project docs](docs/README.md).
