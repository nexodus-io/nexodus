# Deploying the Nexodus Service

This document discusses how to run the control plane for Nexodus.

## Run on Kubernetes

### Deploy using KIND

> **Note**
> This section is only if you want to build the service stack. If you want to attach to a running service, see [Deploying the Nexodus Agent](../user-guide/user-guide.md#deploying-the-nexodus-agent).

You should first ensure that you have `kind`, `kubectl` and [`mkcert`](https://github.com/FiloSottile/mkcert) installed.

If not, you can follow the instructions in the [KIND Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/).

Once you have `kind` installed, you should also follow the instructions [here](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) to prevent errors due to "too many open files".

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

You can recreate that file at any time with the following.

```console
make cacerts
```

In order to join a self-signed Nexodus Service from a remote node or view the Nexodus UI in your dev environment, you will need to install the cert on the remote machine. This is only necessary when the service is self-signed with a domain like we are using with the `try.nexodus.127.0.0.1.nip.io` domain for development.

Add the following host entries to `/etc/hosts` pointing to the IP the kind stack is running on.

```console
<IP of machine running the KIND stack> auth.try.nexodus.127.0.0.1.nip.io api.try.nexodus.127.0.0.1.nip.io try.nexodus.127.0.0.1.nip.io
```

Install [`mkcert`](https://github.com/FiloSottile/mkcert) on the agent node, copy the cert from the service running kind (`.certs/rootCA.pem`) to the remote node you will be joining (or viewing the web UI) and run the following.

```console
CAROOT=$(pwd)/.certs mkcert -install
```

Verify the service by attaching a node using built-in accounts as part of the kubernetes dev overlay build `make run-on-kind` provides.

```console
# from the nexodus repo directory root:
make dist/nexd-linux-amd64
sudo NEXD_LOGLEVEL=debug dist/nexd-linux-amd64 --username admin --password floofykittens  https://try.nexodus.127.0.0.1.nip.io

# or if you wanted to run multiple sandboxed containers:
make run-nexd-container
```

Alternatively, or build the nexctl binary and running a command with it.

```console
make dist/nexctl-linux-amd64
dist/nexctl-linux-amd64 --host https://api.try.nexodus.127.0.0.1.nip.io --username admin --password floofykittens -output json device list
```

For windows, we recommend installing the root certificate via the [MMC snap-in](https://learn.microsoft.com/en-us/troubleshoot/windows-server/windows-security/install-imported-certificates#import-the-certificate-into-the-local-computer-store).
