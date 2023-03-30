# Deploying the Nexodus Service

This document discusses how to run the control plane for Nexodus.

## Run on Kubernetes

### Deploy using KIND

> **Note**
> This section is only if you want to build the controller stack. If you want to attach to a running controller, see [Deploying the Nexodus Agent](../user-guide/user-guide.md#deploying-the-nexodus-agent).

You should first ensure that you have `kind`, `kubectl` and [`mkcert`](https://github.com/FiloSottile/mkcert) installed.

If not, you can follow the instructions in the [KIND Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/).

```console
make run-on-kind
```

This will install:

- `nexodus-dev` kind cluster
- `ingress-nginx` ingress controller
- a rewrite rule in coredns to allow `auth.try.nexodus.127.0.0.1.nip.io` to resolve inside the k8s cluster
- the `nexodus` stack

To bring the cluster down again:

```console
make teardown
```

## HTTPS

The Makefile will install the https certs. You can view the cert in the Nexodus root where you ran the Makefile.

```console
cat .certs/rootCA.pem
```

In order to join a self-signed Nexodus controller from a remote node or view the Nexodus UI in your dev environment, you will need to install the cert on the remote machine. This is only necessary when the controller is self-signed with a domain like we are using with the try.nexodus.127.0.0.1.nip.io domain for development.

Install [`mkcert`](https://github.com/FiloSottile/mkcert) on the agent node, copy the cert from the controller running kind (`.certs/rootCA.pem`) to the remote node you will be joining (or viewing the web UI) and run the following.

```console
CAROOT=$(pwd)/.certs mkcert -install
```

For windows, we recommend installing the root certificate via the [MMC snap-in](https://learn.microsoft.com/en-us/troubleshoot/windows-server/windows-security/install-imported-certificates#import-the-certificate-into-the-local-computer-store).
