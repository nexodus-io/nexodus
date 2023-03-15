# Production and QA Deployments

In order to follow our "Service First" guiding principle, we deploy Nexodus continuously as a service using GitOps.

## Kubernetes Manifests

Our project is deployed to Kubernetes using [kustomize](https://kustomize.io/).
This enables Nexodus to be easily adapted for different deployment scenarios.

The base Nexodus manifests live in `./deploy/nexodus/base`, and we offer a number of overlays:

- Local Development - `./deploy/nexodus/overlays/dev`
- Operate First (QA) - `./deploy/nexodus/overlays/qa`
- Operate First (Production) - `./deploy/nexodus/overlays/prod`

## Build Pipeline

We use GitHub Actions as our build pipeline.
Each Pull Request is gated by automated tests that are run in the `build` workflow.
On each merge to the `main` branch, the `deploy` workflow is triggered.

This workflow:

1. Builds container images and pushes them to quay.io
1. Updates the image tags in `./deploy/nexodus/overlays/qa` and commits this change back to the repository

This ensures that the desired state of our deployments is checked into git.

## Deployment Pipeline

Deployment of Nexodus on OperateFirst is managed by ArgoCD.

ArgoCD will watch this repository for changes and ensure that our deployment is up-to-date.
It will prevent our application from deviating from the desired state.
