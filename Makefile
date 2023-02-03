.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: go-lint apexd

##@ Binaries

.PHONY: apexd
apexd: dist/apexd dist/apexd-linux-arm dist/apexd-linux-amd64 dist/apexd-darwin-amd64 dist/apexd-darwin-arm64 dist/apexd-windows-amd64 ## Build the apexd binary for all architectures

COMMON_DEPS=$(wildcard ./internal/**/*.go) go.sum go.mod

APEXD_DEPS=$(COMMON_DEPS) $(wildcard cmd/apexd/*.go)

APISERVER_DEPS=$(COMMON_DEPS) $(wildcard cmd/apiserver/*.go)

TAG=$(shell git rev-parse HEAD)

dist:
	mkdir -p $@

dist/apexd: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 go build -o $@ ./cmd/apexd

dist/apexd-linux-arm: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o $@ ./cmd/apexd

dist/apexd-linux-amd64: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/apexd

dist/apexd-darwin-amd64: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ ./cmd/apexd

dist/apexd-darwin-arm64: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ ./cmd/apexd

dist/apexd-windows-amd64: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o $@ ./cmd/apexd

dist/apexctl: $(APEXD_DEPS) | dist
	CGO_ENABLED=0 go build -o $@ ./cmd/apexctl

.PHONY: clean
clean: ## clean built binaries
	rm -rf dist

##@ Development

.PHONY: go-lint
go-lint: $(APEXD_DEPS) $(APISERVER_DEPS) ## Lint the go code
	@if ! which golangci-lint 2>&1 >/dev/null; then \
		echo "Please install golangci-lint." ; \
		echo "See: https://golangci-lint.run/usage/install/#local-installation" ; \
		exit 1 ; \
	fi
	golangci-lint run ./...

.PHONY: gen-docs
gen-docs: ## Generate API docs
	swag init -g ./cmd/apiserver/main.go -o ./internal/docs

.PHONY: e2e
e2e: e2eprereqs dist/apexd dist/apexctl test-images ## Run e2e tests
	go test -v --tags=integration ./integration-tests/...

.PHONY: test
test: ## Run unit tests
	go test -v ./...

APEX_LOCAL_IP:=`getent hosts apex.local | awk '{ print $$1 }'`
.PHONY: run-test-container
run-test-container: e2eprereqs dist/apexd dist/apexctl test-images ## Run docker container that you can run apex in
	docker run --rm -it --network bridge \
		--cap-add SYS_MODULE \
		--cap-add NET_ADMIN \
		--cap-add NET_RAW \
		--add-host apex.local:$(APEX_LOCAL_IP) \
		--add-host api.apex.local:$(APEX_LOCAL_IP) \
		--add-host auth.apex.local:$(APEX_LOCAL_IP) \
		--entrypoint /bin/bash quay.io/apex/test:ubuntu

##@ Container Images

.PHONY: test-images
test-images: ## Create test images for e2e
	docker build -f Containerfile.test -t quay.io/apex/test:alpine --target alpine .
	docker build -f Containerfile.test -t quay.io/apex/test:fedora --target fedora .
	docker build -f Containerfile.test -t quay.io/apex/test:ubuntu --target ubuntu .

.PHONY: e2eprereqs
e2eprereqs:
	@if [ -z "$(shell which kind)" ]; then \
		echo "Please install kind and then start the kind dev environment." ; \
		echo "https://kind.sigs.k8s.io/" ; \
		echo "  $$ hack/kind/kind.sh up" ; \
		echo "  $$ hack/kind/kind.sh cacert" ; \
		exit 1 ; \
	fi
	@if [ -z "$(findstring apex-dev,$(shell kind get clusters))" ]; then \
		echo "Please start the kind dev environment." ; \
		echo "  $$ hack/kind/kind.sh up" ; \
		echo "  $$ hack/kind/kind.sh cacert" ; \
		exit 1 ; \
	fi

.PHONY: image-frontend
image-frontend:
	docker build -f Containerfile.frontend -t quay.io/apex/frontend:$(TAG) .
	docker tag quay.io/apex/frontend:$(TAG) quay.io/apex/frontend:latest

.PHONY: image-apiserver
image-apiserver:
	docker build -f Containerfile.apiserver -t quay.io/apex/apiserver:$(TAG) .
	docker tag quay.io/apex/apiserver:$(TAG) quay.io/apex/apiserver:latest

.PHONY: image-apex ## Build the apex agent image
image-apex:
	docker build -f Containerfile.apex -t quay.io/apex/apex:$(TAG) .
	docker tag quay.io/apex/apex:$(TAG) quay.io/apex/apex:latest

.PHONY: images
images: image-frontend image-apiserver ## Create container images

##@ Kubernetes - kind dev environment

.PHONY: run-on-kind
run-on-kind: setup-kind deploy-operators load-images deploy cacerts ## Setup a kind cluster and deploy apex on it

.PHONY: teardown
teardown: ## Teardown the kind cluster
	@kind delete cluster --name apex-dev

.PHONY: setup-kind
setup-kind: teardown ## Create a kind cluster with ingress enabled, but don't install apex.
	@kind create cluster --config ./deploy/kind.yaml
	@kubectl cluster-info --context kind-apex-dev
	@kubectl apply -f ./deploy/kind-ingress.yaml

.PHONY: deploy-apex-agent ## Deply the apex agent in the kind cluster
deploy-apex-agent: image-apex
	@kind load --name apex-dev docker-image quay.io/apex/apex:latest
	@cp deploy/apex-client/overlays/dev/kustomization.yaml.sample deploy/apex-client/overlays/dev/kustomization.yaml
	@sed -i -e "s/<APEX_CONTROLLER_IP>/$(APEX_LOCAL_IP)/" deploy/apex-client/overlays/dev/kustomization.yaml
	@sed -i -e "s/<APEX_CONTROLLER_CERT>/$(shell kubectl get secret -n apex apex-ca-key-pair -o json | jq -r '.data."ca.crt"')/" deploy/apex-client/overlays/dev/kustomization.yaml
	@kubectl apply -k ./deploy/apex-client/overlays/dev

##@ Kubernetes - work with an existing cluster (kind dev env or another one)

.PHONY: deploy-operators
deploy-operators: deploy-certmanager deploy-pgo wait-for-readiness ## Deploy all operators and wait for readiness

.PHONY: deploy-certmanager
deploy-certmanager: # Deploy cert-manager
	@kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml

.PHONY: deploy-pgo
deploy-pgo: # Deploy crunchy-data postgres operator
	@kubectl apply -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/namespace
	@kubectl apply --server-side -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/default

.PHONY: wait-for-readiness
wait-for-readiness: # Wait for operators to be installed
	@kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=5m
	@kubectl rollout status -n cert-manager deploy/cert-manager --timeout=5m
	@kubectl rollout status -n cert-manager deploy/cert-manager-webhook --timeout=5m
	@kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=5m
	@kubectl wait --for=condition=Ready pods --all -n postgres-operator --timeout=5m

.PHONY: deploy
deploy: ## Deploy a development apex stack onto a kubernetes cluster
	@kubectl create namespace apex
	@kubectl apply -k ./deploy/apex/overlays/dev
	@kubectl wait --for=condition=Ready pods --all -n apex -l app.kubernetes.io/part-of=apex --timeout=15m

.PHONY: undeploy
undeploy: ## Remove the apex stack from a kubernetes cluster
	@kubectl delete namespace apex

.PHONY: load-images
load-images: images ## Load images onto kind
	@kind load --name apex-dev docker-image quay.io/apex/apiserver:latest
	@kind load --name apex-dev docker-image quay.io/apex/frontend:latest

.PHONY: redeploy
redeploy: load-images ## Redploy apex after images changes
	@kubectl rollout restart deploy/apiserver -n apex
	@kubectl rollout restart deploy/frontend -n apex

.PHONY: recreate-db
recreate-db: recreate-db ## Delete and bring up a new apex database
	@kubectl delete -n apex deploy/apiserver postgrescluster/database deploy/ipam
	@kubectl apply -k ./deploy/apex/overlays/dev

.PHONY: cacerts
cacerts: ## Install the Self-Signed CA Certificate
	@mkdir -p $(CURDIR)/.certs
	@kubectl get secret -n apex apex-ca-key-pair -o json | jq -r '.data."ca.crt"' | base64 -d > $(CURDIR)/.certs/rootCA.pem
	@CAROOT=$(CURDIR)/.certs mkcert -install
