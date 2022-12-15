Deployment
==========

In order to follow our "Service First" guiding principle, we deploy Apex continuously as a service on [OperateFirst](https://operate-first.cloud) using GitOps.

## Kubernetes Manifests

Our project is deployed to Kubernetes using [kustomize](https://kustomize.io/).
This enables Apex to be easily adapted for different deployment scenarios.

The base apex manifests live in `./deploy/apex/base`, and we offer two overlays
- Development - `./deploy/apex/overlays/dev`
- Operate First - `./deploy/apex/overlays/operate-first`

## Build Pipeline

We use GitHub Actions as our build pipeline.
Each Pull Request is gated by automated tests that are run in the `build` workflow.
On each merge to the `main` branch, the `deploy` workflow is triggered.

This workflow:

1. Builds container images and pushes them to quay.io
1. Updates the image tags in `./deploy/apex/overlays/operate-first` and commits this change back to the repository

This ensures that the desired state of our OperateFirst deployment is checked in to git.

## Deployment Pipeline

Deployment of Apex on OperateFirst is managed by [ArgoCD](https://argocd.operate-first.cloud/applications/apex-smaug).
The ArgoCD project configuration lives [here](https://github.com/operate-first/apps/blob/master/argocd/overlays/moc-infra/projects/apex.yaml), and our ArgoCD Application configuration lives [here](https://github.com/operate-first/apps/blob/master/argocd/overlays/moc-infra/applications/envs/moc/smaug/apex/apex.yaml).

ArgoCD will watch this repository for changes and ensure that our deployment is up-to-date.
It will prevent our application deviating from the desired state.
