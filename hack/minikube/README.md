# Developer Environment setup with minikube

*Note* : Please make sure you have minikube and kubectl installed on the machine where you are deploying Apex stack.

## Deploy Apex 
Clone the apex repo:

```shell
git clone https://github.com/redhat-et/apex.git
cd ./hack/minikube/
```
To deploy Apex stack please run the following script. HOST_IP_ADDRESS is the address of the **reachable** IP address of the machine. If you are running it on your local laptop, it should be 127.0.0.1 and if you are running it somewhere on your Cloud VM, it should be the public IP address (stun address). 

```shell
./minikube.sh -u <HOST_IP_ADDRESS>
e.g ./minikube -u 1270.0.1
```
Start the minikube tunnel to route traffic to the ingress.

```shell
sudo minikube tunnel
```

Above script deploys multiple deployment related to various component of the Apex stack. It also deploys the Ingress resources to route the external traffic to the services.
If you want to hack the manifest files, they are present in `apex/deploy/`. 

## Remove Apex

```shell
cd ./hack/minikube/

./minikube.sh  -d
```


