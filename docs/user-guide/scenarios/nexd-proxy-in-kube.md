# Nexd Proxy in Kubernetes

The default mode of running `nexd` requires privileges to create a network
device. This prevents using it in a container environment without the ability to
grant those extra privileges. [`nexd proxy`](../../development/design/userspace-mode.md)
addresses this by allowing `nexd` to operate as an L4 proxy. However, the
configuration of the L4 proxy is done in terms of port forwarding rules. For an
application developer using Kubernetes, it is most convenient to define the
desired network connectivity in terms of Kubernetes constructs. This proposal is
to explore some approaches for using Nexodus to achieve connectivity to and from
application resources in Kubernetes.

For now, this document includes scenarios and sample Kubernetes manifests.
Later, we will provide an example of an application that automates these
scenarios. For a more complete discussion of these plans, see the [nexlink
design document](../../development/design/nexlink.md).

## Demo 1 - Exposing a Kubernetes Service to a Nexodus Organization

In this demo, we will run `nexd proxy` in a Pod that will forward
connections to a Service inside of a cluster. This will allow any device within
a Nexodus organization to reach this service, no matter where they are.

The Pod running `nexd proxy` is using a single ingress proxy rule:

```sh
nexd proxy --ingress tcp:80:nginx-service.nexd-proxy-demo1.svc.cluster.local:80 https://try.nexodus.io
```

```mermaid
flowchart TD
 linkStyle default interpolate basis
 device1[Remote device running nexd<br/><br/>IP: 100.100.0.1<br/><br/>Initiates connection to 100.100.0.2:80]-->|tunnel|network{Nexodus Network<br/><br/>100.100.0.0/16}
 network-->|tunnel|container[Pod running nexd in proxy mode.<br/><br/>Nexodus IP: 100.100.0.2<br/>Pod IP: 10.10.10.151<br/><br/>Accepts connections on 100.100.0.2:80 and forwards to nginx-service.nexd-proxy-demo1.svc.cluster.local:80]

 subgraph Kubernetes Cluster
 container-->|tcp|dest(Kubernetes Service<br/><br/>Name: nginx-service)
 dest-->|tcp|pod(Nginx Pod)
 dest-->|tcp|pod2(Nginx Pod)
 end
```

To implement this scenario you will need a Kubernetes cluster and a Nexodus
Service that allows user/password authentication.

First, set a few variables that we will use for this demo. The username and
password will be used by `nexd proxy` to authenticate with the Nexodus Service.

```console
NAMESPACE=nexd-proxy-demo1
USERNAME=username
PASSWORD=password
```

Start by creating a namespace for the demo:

```console
kubectl create namespace "${NAMESPACE}"
```

Next, create a Secret that contains the username and password that `nexd proxy`
will use to authenticate with the Nexodus Service.

```console
kubectl create secret generic nexodus-credentials \
    --from-literal=username="${USERNAME}" \
    --from-literal=password="${PASSWORD}" -n "${NAMESPACE}"
```

We also need a Secret to hold the wireguard keys used by `nexd`. If you need to
create the keys, you can use these commands:

```console
wg genkey | tee private.key | wg pubkey > public.key
```

Once you have the `private.key` and `public.key` files, you can create a Secret
for them.

```console
kubectl create secret generic wireguard-keys \
    --from-literal=private.key="$(cat private.key)" \
    --from-literal=public.key="$(cat public.key)" -n "${NAMESPACE}"
```

Next we need to create a target Service that we will be exposing from the
Kubernetes cluster. For this example, we will a Deployment of nginx with two
replicas. Each will serve up a file giving its Pod name.

Save the yaml to a file called `nginx.yaml` and then apply it to your cluster.

```console
kubectl apply -n "${NAMESPACE}" -f nginx.yaml
```

```yaml
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      volumes:
      - name: shared-data
        emptyDir: {}
      initContainers:
      - name: init-nginx
        image: nginx
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        command: ["sh", "-c", "echo \"Hello from $POD_NAME\" > /usr/share/nginx/html/index.html"]
        volumeMounts:
        - name: shared-data
          mountPath: /usr/share/nginx/html
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        volumeMounts:
        - name: shared-data
          mountPath: /usr/share/nginx/html
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
spec:
  selector:
    app: nginx
  ports:
  - name: http
    port: 80
    targetPort: 80
  type: ClusterIP
```

The final step is to create the Deployment for `nexd proxy`. Note that if you
changed the Nexodus Service URL or the namespace used for this demo, you will
need to update the arguments given to `nexd` in this Deployment.

Save this yaml to a file called `demo1.yaml` and then apply it to your cluster.

```console
kubectl apply -n "${NAMESPACE}" -f demo1.yaml
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nexd-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nexd-proxy
  template:
    metadata:
      labels:
        app: nexd-proxy
    spec:
      containers:
      - name: my-container
        image: quay.io/nexodus/nexd
        command: ["sh"]
        args: ["-c", "ln -s /etc/wireguard/private.key /private.key; ln -s /etc/wireguard/public.key /public.key; nexd proxy --ingress tcp:80:nginx-service.nexd-proxy-demo1.svc.cluster.local:80 https://try.nexodus.io"]
        env:
        - name: NEXD_USERNAME
          valueFrom:
            secretKeyRef:
              name: nexodus-credentials
              key: username
        - name: NEXD_PASSWORD
          valueFrom:
            secretKeyRef:
              name: nexodus-credentials
              key: password
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        volumeMounts:
        - name: wireguard-keys
          mountPath: /etc/wireguard/
      volumes:
      - name: wireguard-keys
        secret:
          secretName: wireguard-keys
```
