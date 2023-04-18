# Debugging Nexodus

Tips and tricks to help you debug the various Nexodus components.

## About Telepresence

Developing services deployed in Kubernetes can be tricky since it can be a bit slow to deploy changes into Kubernetes and even hard to remote debug.  Telepresence allows you to reroute network traffic going in and out of a pod to your local machine so that you can more easily have a fast development loop of the software running in that pod.

Many of the tips in this document require you to first install [telepresence](https://www.telepresence.io/).   O

## Debugging/Developing the `apiserver`

Once you have the Nexodus service [running in Kind](../deployment/nexodus-service.md#deploy-using-kind), run:

    make debug-apiserver

This will create a `apiserver-envs.json` file that contains all the environment variables that you should set when your run the apiserver locally with a debugger.  If your using an [GoLand](https://www.jetbrains.com/go/) or [IDEA](https://www.jetbrains.com/idea/) ide, install the [EnvFile Plugin](https://plugins.jetbrains.com/plugin/7861-envfile).  This will allow you automatically read and set the environment variables up when you launch and debug the api server.

![env file screenshot](./env-file-screenshot.png)

Once you run the `apiserver` locally, requests against that Nexodus service should result in http requests being executed against your locally running `apiserver`.

To stop routing traffic from the apiserver pod to your machine, run:

    make debug-apiserver-stop

## Debugging/Developing the `frontend`

Once you have the Nexodus service [running in Kind](../deployment/nexodus-service.md#deploy-using-kind), run:

    make debug-frontend

This will start a development vite server locally so that any local changes to the ui sources can instantly reloaded in your browser.  
This uses Telepresence, so ignore the URLs on the screen from vite, and instead connect to [https://try.nexodus.127.0.0.1.nip.io/](https://try.nexodus.127.0.0.1.nip.io/)

To stop routing traffic from the frontend pod to your machine, run:

    make debug-frontend-stop
