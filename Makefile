.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

#
# If you want to see the full commands, run:
#   NOISY_BUILD=y make
#
ifeq ($(NOISY_BUILD),)
    ECHO_PREFIX=@
    CMD_PREFIX=@
    PIPE_DEV_NULL=> /dev/null 2> /dev/null
	SWAG_ARGS?=--quiet
else
    ECHO_PREFIX=@\#
    CMD_PREFIX=
    PIPE_DEV_NULL=
	SWAG_ARGS?=
endif

NEXODUS_VERSION?=$(shell date +%Y.%m.%d)
NEXODUS_RELEASE?=$(shell git describe --always --exclude qa --exclude prod)
NEXODUS_GCFLAGS?=

NEXODUS_BUILD_PROFILE?=dev
NEXODUS_LDFLAGS:=$(NEXODUS_LDFLAGS) -X main.Version=$(NEXODUS_VERSION)-$(NEXODUS_RELEASE)
ifeq ($(NEXODUS_BUILD_PROFILE),dev)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://try.nexodus.127.0.0.1.nip.io
else ifeq ($(NEXODUS_BUILD_PROFILE),qa)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://qa.nexodus.io
else ifeq ($(NEXODUS_BUILD_PROFILE),prod)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://try.nexodus.io
endif

SWAGGER_YAML:=internal/docs/swagger.yaml

# Crunchy DB operator does not work well on arm64, use an different overlay to work around it.
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),arm64)
	OVERLAY?=arm64
else
	OVERLAY?=dev
endif

##@ All

.PHONY: all
all: gen-openapi-client generate go-lint yaml-lint md-lint ui-lint opa-lint nexd nexctl ## Run linters and build nexd

##@ Binaries

.PHONY: nexd
nexd: dist/nexd dist/nexd-linux-arm dist/nexd-linux-amd64 dist/nexd-darwin-amd64 dist/nexd-darwin-arm64 dist/nexd-windows-amd64.exe ## Build the nexd binary for all architectures

.PHONY: nexctl
nexctl: dist/nexctl dist/nexctl-linux-arm dist/nexctl-linux-amd64 dist/nexctl-darwin-amd64 dist/nexctl-darwin-arm64 dist/nexctl-windows-amd64.exe ## Build the nexctl binary for all architectures

# Use go list to find all the go files that make up a binary.
NEXD_DEPS:=     $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/nexd)
NEXCTL_DEPS:=   $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/nexctl)
APISERVER_DEPS:=$(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/apiserver)
NEX_ALL_GO:=    $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./...)

TAG=$(shell git rev-parse HEAD)

dist:
	$(CMD_PREFIX) mkdir -p $@

dist/nexd: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(subst https://,https://api.,$(NEXODUS_LDFLAGS))" -o $@ ./cmd/nexctl

dist/nexd-%: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl-%: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(subst https://,https://api.,$(NEXODUS_LDFLAGS))" -o $@ ./cmd/nexctl

.PHONY: clean
clean: ## clean built binaries
	$(CMD_PREFIX) rm -rf dist

##@ Development

.PHONY: go-lint
go-lint: dist/.go-lint-linux dist/.go-lint-darwin dist/.go-lint-windows ## Lint the go code

.PHONY: go-lint-prereqs
go-lint-prereqs:
	$(CMD_PREFIX) if ! which golangci-lint >/dev/null 2>&1; then \
		echo "Please install golangci-lint." ; \
		echo "See: https://golangci-lint.run/usage/install/#local-installation" ; \
		exit 1 ; \
	fi

dist/.go-lint-%: $(NEX_ALL_GO) | go-lint-prereqs gen-docs dist gen-openapi-client $(wildcard internal/api/public/*.go)
	$(ECHO_PREFIX) printf "  %-12s GOOS=$(word 3,$(subst -, ,$@))\n" "[GO LINT]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 3,$(subst -, ,$@)) GOARCH=amd64 \
		golangci-lint run --build-tags integration --timeout 5m ./...
	$(CMD_PREFIX) touch $@

.PHONY: yaml-lint
yaml-lint: dist/.yaml-lint ## Lint the yaml files

# If gen-docs needs to run, make sure it goes first as it generates internal/docs/swagger.yaml
dist/.yaml-lint: $(wildcard */**/*.yaml) | gen-docs dist
	$(CMD_PREFIX) if ! which yamllint >/dev/null 2>&1; then \
		echo "Please install yamllint." ; \
		echo "See: https://yamllint.readthedocs.io/en/stable/quickstart.html" ; \
		exit 1 ; \
	fi
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[YAML LINT]"
	$(CMD_PREFIX) yamllint -c .yamllint.yaml deploy --strict
	$(CMD_PREFIX) touch $@

.PHONY: md-lint
md-lint: dist/.md-lint ## Lint markdown files

dist/.md-lint: $(wildcard */**/*.md) | dist
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[MD LINT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir docker.io/davidanson/markdownlint-cli2:v0.6.0 > /dev/null
	$(CMD_PREFIX) touch $@

.PHONY: ui-lint
ui-lint: dist/.ui-lint ## Lint the UI source

dist/.ui-lint: $(filter-out $(wildcard ui/node_modules/*),$(wildcard ui/*) $(wildcard ui/**/*)) | dist
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[UI LINT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir tmknom/prettier --check /workdir/ui/src/ >/dev/null
	$(CMD_PREFIX) touch $@

policies=$(wildcard internal/routers/*.rego)

.PHONY: opa-lint
opa-lint: dist/.opa-lint ## Lint the OPA policies
dist/.opa-lint: $(policies) | dist
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[OPA LINT]"
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest fmt --fail $(policies) $(PIPE_DEV_NULL)
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest test -v $(policies) $(PIPE_DEV_NULL)
	$(CMD_PREFIX) touch $@

.PHONY: gen-docs
gen-docs: $(SWAGGER_YAML) ## Generate API docs
.PHONY: openapi-lint
openapi-lint: dist/.openapi-lint ## Lint the OpenAPI document
.PHONY: openapi-lint
dist/.openapi-lint: internal/docs/swagger.yaml | dist
	$(ECHO_PREFIX) printf "  %-12s \n" "[OPENAPI LINT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/src openapitools/openapi-generator-cli:v6.5.0 \
		validate -i /src/internal/docs/swagger.yaml
	$(CMD_PREFIX) touch $@

$(SWAGGER_YAML): $(APISERVER_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s ./cmd/apiserver/main.go\n" "[API DOCS]"
	$(CMD_PREFIX) docker run \--platform linux/x86_64 --rm \
		-v $(CURDIR):/workdir -w /workdir \
		ghcr.io/swaggo/swag:v1.8.10 \
		/root/swag init $(SWAG_ARGS) -g ./cmd/apiserver/main.go -o ./internal/docs

.PHONY: gen-openapi-client
gen-openapi-client: internal/api/public/client.go ## Generate the OpenAPI Client
internal/api/public/client.go: internal/docs/swagger.yaml | dist
	$(ECHO_PREFIX) printf "  %-12s internal/docs/swagger.yaml\n" "[OPENAPI CLIENT GEN]"
	$(CMD_PREFIX) rm -f $(shell find internal/api/public | grep .go | grep -v _custom.go)
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/src openapitools/openapi-generator-cli:v6.5.0 \
		generate -i /src/internal/docs/swagger.yaml -g go \
		--package-name public \
		-o /src/internal/api/public \
		-t /src/hack/openapi-templates \
		--ignore-file-override /src/.openapi-generator-ignore $(PIPE_DEV_NULL)
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO FMT]"
	$(CMD_PREFIX) [ -z "$(shell gofmt -l .)" ] || gofmt -w .

internal/api/public/%.go: internal/api/public/client.go

.PHONY: opa-fmt
opa-fmt: ## Lint the OPA policies
	$(ECHO_PREFIX) printf "  %-12s \n" "[OPA FMT]"
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest fmt --write $(policies)


.PHONY: ui-fmt
ui-fmt: dist/.ui-fmt ## Format the UI sources
dist/.ui-fmt: $(wildcard ui/*) $(wildcard ui/src/**) | dist
	$(ECHO_PREFIX) printf "  %-12s \n" "[UI FMT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir tmknom/prettier --write /workdir/ui/src/ $(PIPE_DEV_NULL)
	$(CMD_PREFIX) touch $@

.PHONY: generate
generate: dist/.generate ## Run all code generators and formatters

dist/.generate: $(SWAGGER_YAML) dist/.ui-fmt | dist
	$(ECHO_PREFIX) printf "  %-12s \n" "[MOD TIDY]"
	$(CMD_PREFIX) go mod tidy

	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO FMT]"
	$(CMD_PREFIX) [ -z "$(shell gofmt -l .)" ] || gofmt -w .
	$(CMD_PREFIX) touch $@

.PHONY: e2e
e2e: e2eprereqs dist/nexd dist/nexctl image-nexd ## Run e2e tests
	go test -race -v --tags=integration ./integration-tests/... $(shell [ -z "$$NEX_TEST" ] || echo "-run $$NEX_TEST" )

.PHONY: e2e-podman
e2e-podman: ## Run e2e tests on podman
	go test -c -v --tags=integration ./integration-tests/...
	sudo NEXODUS_TEST_PODMAN=1 TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED=true ./integration-tests.test -test.v

.PHONY: test
test: ## Run unit tests
	go test -v ./...

telepresence_%: telepresence-prereqs
	$(CMD_PREFIX) if [ "$(shell telepresence status --output json | jq .user_daemon.status -r)" != "Connected" ]; then \
		telepresence helm install 2> /dev/null || true ;\
		telepresence connect ;\
	fi
	$(CMD_PREFIX) if [ -z "$(shell telepresence status --output json | jq '.user_daemon.intercepts[]|select(.name == "$(word 2,$(subst _, ,$(basename $@)))-nexodus")' 2> /dev/null)" ]; then \
		telepresence intercept -n nexodus $(word 2,$(subst _, ,$(basename $@))) --port $(word 3,$(subst _, ,$(basename $@))) --env-json=$(word 2,$(subst _, ,$(basename $@)))-envs.json ;\
		echo "=======================================================================================" ;\
		echo ;\
		echo "   Start the $(word 2,$(subst _, ,$(basename $@))) locally with a debugger with the env variables" ;\
		echo "   with the values found in: $(word 2,$(subst _, ,$(basename $@)))-envs.json" ;\
		echo ;\
		echo "   Hint: use the IDEA EnvFile plugin https://plugins.jetbrains.com/plugin/7861-envfile" ;\
		echo ;\
	fi

.PHONY: debug-apiserver
debug-apiserver: telepresence_apiserver_8080 ## Use telepresence to debug the apiserver deployment
.PHONY: debug-apiserver-stop
debug-apiserver-stop: telepresence-prereqs ## Stop using telepresence to debug the apiserver deployment
	$(CMD_PREFIX) telepresence leave apiserver-nexodus

dist/.npm-install:
	$(CMD_PREFIX) cd ui; npm install
	$(CMD_PREFIX) touch $@

.PHONY: debug-frontend
debug-frontend: telepresence_frontend_3000 dist/.npm-install ## Use telepresence to debug the frontend deployment
	$(CMD_PREFIX) cd ui; npm start

.PHONY: debug-frontend-stop
debug-frontend-stop: telepresence-prereqs ## Stop using telepresence to debug the frontend deployment
	$(CMD_PREFIX) telepresence leave frontend-nexodus

NEXODUS_LOCAL_IP:=`go run ./hack/localip`
.PHONY: run-nexd-container
run-nexd-container: image-nexd ## Run a container that you can run nexodus in
	$(CMD_PREFIX) docker run --rm -it --network bridge \
		--cap-add SYS_MODULE \
		--cap-add NET_ADMIN \
		--cap-add NET_RAW \
		--sysctl net.ipv4.ip_forward=1 \
		--sysctl net.ipv6.conf.all.disable_ipv6=0 \
		--sysctl net.ipv6.conf.all.forwarding=1 \
		--add-host try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host api.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host auth.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--mount type=bind,source=$(shell pwd)/.certs,target=/.certs,readonly \
		quay.io/nexodus/nexd:latest /update-ca.sh

.dev-container: Containerfile.dev
	$(CMD_PREFIX) docker build -f Containerfile.dev -t quay.io/nexodus/dev:latest .
	$(CMD_PREFIX) touch $@

.PHONY: run-dev-container
run-dev-container: .dev-container ## Run docker container that you can develop and run nexodus in
	$(CMD_PREFIX) docker run --rm -it --network bridge \
		--cap-add SYS_MODULE \
		--cap-add NET_ADMIN \
		--cap-add NET_RAW \
		--add-host try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host api.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host auth.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--mount type=bind,source=$(shell pwd)/.certs,target=/.certs,readonly \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(CURDIR):$(CURDIR) \
		--workdir $(CURDIR) \
		quay.io/nexodus/dev:latest

.PHONY: run-sql-apiserver
run-sql-apiserver: ## runs a command line SQL client to interact with the apiserver database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl exec -it -n nexodus \
		$(shell kubectl get pods -l postgres-operator.crunchydata.com/role=master -o name -n nexodus) \
		-c database -- psql apiserver
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/postgres -c postgres -- psql -U apiserver apiserver
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/cockroachdb -- cockroach sql --insecure --user apiserver --database apiserver
endif

.PHONY: run-sql-ipam
run-sql-ipam: ## runs a command line SQL client to interact with the ipam database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl exec -it -n nexodus \
		$(shell kubectl get pods -l postgres-operator.crunchydata.com/role=master -o name -n nexodus) \
		-c database -- psql ipam
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/postgres -c postgres -- psql -U ipam ipam
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/cockroachdb -- cockroach sql --insecure --user ipam --database ipam
endif

.PHONY: run-sql-keycloak
run-sql-keycloak: ## runs a command line SQL client to interact with the keycloak database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl exec -it -n nexodus \
		$(shell kubectl get pods -l postgres-operator.crunchydata.com/role=master -o name -n nexodus) \
		-c database -- psql keycloak
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/postgres -c postgres -- psql -U keycloak keycloak
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/cockroachdb -- cockroach sql --insecure --user keycloak --database keycloak
endif


.PHONY: clear-db
clear-db:
	$(CMD_PREFIX) kubectl scale deployment apiserver --replicas=0 -n nexodus $(PIPE_DEV_NULL)
	$(CMD_PREFIX) kubectl rollout status deploy/apiserver -n nexodus --timeout=5m $(PIPE_DEV_NULL)
	$(ECHO_PREFIX) printf "  %-12s \n" "[DROP TABLE IF EXISTS] apiserver_migrations"
	$(CMD_PREFIX) echo "\
		DROP TABLE IF EXISTS invitations;\
		DROP TABLE IF EXISTS devices;\
		DROP TABLE IF EXISTS user_organizations;\
		DROP TABLE IF EXISTS organizations;\
		DROP TABLE IF EXISTS users;\
		DROP TABLE IF EXISTS apiserver_migrations;\
		" | make run-sql-apiserver $(PIPE_DEV_NULL)
	$(CMD_PREFIX) kubectl scale deployment apiserver --replicas=1 -n nexodus $(PIPE_DEV_NULL)
	$(CMD_PREFIX) kubectl rollout status deploy/apiserver -n nexodus --timeout=5m

##@ Container Images

.PHONY: e2eprereqs
e2eprereqs:
	$(CMD_PREFIX) if [ -z "$(shell which kind)" ]; then \
		echo "Please install kind and then start the kind dev environment." ; \
		echo "https://kind.sigs.k8s.io/" ; \
		echo "  $$ make run-on-kind" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) if [ -z "$(findstring nexodus-dev,$(shell kind get clusters))" ]; then \
		echo "Please start the kind dev environment." ; \
		echo "  $$ make run-on-kind" ; \
		exit 1 ; \
	fi

.PHONY: telepresence-prereqs
telepresence-prereqs: e2eprereqs
	@if [ -z "$(shell which telepresence)" ]; then \
		echo "Please install telepresence first" ; \
		echo "https://www.telepresence.io/docs/latest/quick-start/" ; \
		exit 1 ; \
	fi

.PHONY: image-frontend
image-frontend:
	docker build -f Containerfile.frontend -t quay.io/nexodus/frontend:$(TAG) .
	docker tag quay.io/nexodus/frontend:$(TAG) quay.io/nexodus/frontend:latest

.PHONY: image-apiserver
image-apiserver:
	docker build -f Containerfile.apiserver -t quay.io/nexodus/apiserver:$(TAG) .
	docker tag quay.io/nexodus/apiserver:$(TAG) quay.io/nexodus/apiserver:latest

.PHONY: image-nexd ## Build the nexodus agent image
image-nexd: dist/.image-nexd
dist/.image-nexd: $(NEXD_DEPS) $(NEXCTL_DEPS) Containerfile.nexd | dist
	$(CMD_PREFIX) docker build -f Containerfile.nexd -t quay.io/nexodus/nexd:$(TAG) .
	$(CMD_PREFIX) docker tag quay.io/nexodus/nexd:$(TAG) quay.io/nexodus/nexd:latest
	$(CMD_PREFIX) touch $@

.PHONY: image-ipam ## Build the IPAM image
image-ipam:
	docker build -f Containerfile.ipam -t quay.io/nexodus/go-ipam:$(TAG) .
	docker tag quay.io/nexodus/go-ipam:$(TAG) quay.io/nexodus/go-ipam:latest


.PHONY: image-envsubst ## Build the IPAM image
image-envsubst:
	docker build -f Containerfile.envsubst -t quay.io/nexodus/envsubst:$(TAG) .
	docker tag quay.io/nexodus/envsubst:$(TAG) quay.io/nexodus/envsubst:latest

.PHONY: images
images: image-frontend image-apiserver image-ipam image-envsubst ## Create container images

##@ Kubernetes - kind dev environment

.PHONY: run-on-kind
run-on-kind: setup-kind deploy-operators images load-images deploy cacerts ## Setup a kind cluster and deploy nexodus on it

.PHONY: teardown
teardown: ## Teardown the kind cluster
	$(CMD_PREFIX) kind delete cluster --name nexodus-dev

.PHONY: setup-kind
setup-kind: teardown ## Create a kind cluster with ingress enabled, but don't install nexodus.
	$(CMD_PREFIX) kind create cluster --config ./deploy/kind.yaml
	$(CMD_PREFIX) kubectl cluster-info --context kind-nexodus-dev
	$(CMD_PREFIX) kubectl apply -f ./deploy/kind-ingress.yaml

.PHONY: deploy-nexodus-agent ## Deply the nexodus agent in the kind cluster
deploy-nexodus-agent: image-nexd
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/nexd:latest
	$(CMD_PREFIX) cp deploy/nexodus-client/overlays/dev/kustomization.yaml.sample deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) sed -i -e "s/<NEXODUS_CONTROLLER_IP>/$(NEXODUS_LOCAL_IP)/" deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) sed -i -e "s/<NEXODUS_CONTROLLER_CERT>/$(shell kubectl get secret -n nexodus nexodus-ca-key-pair -o json | jq -r '.data."ca.crt"')/" deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) kubectl apply -k ./deploy/nexodus-client/overlays/dev

##@ Kubernetes - work with an existing cluster (kind dev env or another one)

.PHONY: deploy-operators
deploy-operators: deploy-certmanager deploy-pgo  ## Deploy all operators and wait for readiness

.PHONY: deploy-certmanager
deploy-certmanager: # Deploy cert-manager
	$(CMD_PREFIX) kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml

CRUNCHY_REVISION?=f1766db0b50ad2ae8ff35a599a16e11eefbd9f9c
.PHONY: deploy-pgo
deploy-pgo: # Deploy crunchy-data postgres operator
	$(CMD_PREFIX) kubectl apply -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/namespace?ref=$(CRUNCHY_REVISION)
	$(CMD_PREFIX) kubectl apply --server-side -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/default?ref=$(CRUNCHY_REVISION)

.PHONY: deploy-cockroach-operator
deploy-cockroach-operator: ## Deploy cockroach operator
	$(CMD_PREFIX) kubectl apply -k https://github.com/CrunchyData/postgres-operator-examples/kustomize/install/namespace
	$(CMD_PREFIX) kubectl apply -f https://raw.githubusercontent.com/cockroachdb/cockroach-operator/v2.10.0/install/crds.yaml
	$(CMD_PREFIX) kubectl apply -f https://raw.githubusercontent.com/cockroachdb/cockroach-operator/v2.10.0/install/operator.yaml
	$(CMD_PREFIX) kubectl wait --for=condition=Available --timeout=5m -n cockroach-operator-system deploy/cockroach-operator-manager
	$(CMD_PREFIX) ./hack/wait-for-cockroach-operator-ready.sh

.PHONY: use-cockroach
use-cockroach: deploy-cockroach-operator ## Recreate the database with a Cockroach based server
	$(CMD_PREFIX) OVERLAY=cockroach make recreate-db

.PHONY: use-crunchy
use-crunchy: ## Recreate the database with a Crunchy based postgres server
	$(CMD_PREFIX) OVERLAY=dev make recreate-db

.PHONY: use-postgres
use-postgres: ## Recreate the database with a simple Postgres server
	$(CMD_PREFIX) OVERLAY=arm64 make recreate-db

.PHONY: wait-for-readiness
wait-for-readiness: # Wait for operators to be installed
	$(CMD_PREFIX) kubectl rollout status -n cert-manager deploy/cert-manager --timeout=5m
	$(CMD_PREFIX) kubectl rollout status -n cert-manager deploy/cert-manager-webhook --timeout=5m
	$(CMD_PREFIX) kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=5m
	$(CMD_PREFIX) kubectl wait --for=condition=Ready pods --all -n postgres-operator --timeout=5m
	$(CMD_PREFIX) ./hack/wait-for-resoruce-exists.sh secrets -n ingress-nginx ingress-nginx-admission
	$(CMD_PREFIX) kubectl rollout restart deployment ingress-nginx-controller -n ingress-nginx
	$(CMD_PREFIX) kubectl rollout status deployment ingress-nginx-controller -n ingress-nginx --timeout=5m

.PHONY: deploy
deploy: wait-for-readiness ## Deploy a development nexodus stack onto a kubernetes cluster
	$(CMD_PREFIX) kubectl create namespace nexodus
	$(CMD_PREFIX) kubectl apply -k ./deploy/nexodus/overlays/$(OVERLAY)
	$(CMD_PREFIX) OVERLAY=$(OVERLAY) make init-db
	$(CMD_PREFIX) kubectl wait --for=condition=Ready pods --all -n nexodus -l app.kubernetes.io/part-of=nexodus --timeout=15m

.PHONY: undeploy
undeploy: ## Remove the nexodus stack from a kubernetes cluster
	$(CMD_PREFIX) kubectl delete namespace nexodus

.PHONY: load-images
load-images: ## Load images onto kind
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/apiserver:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/frontend:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/go-ipam:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/envsubst:latest

.PHONY: redeploy
redeploy: images load-images ## Redeploy nexodus after images changes
	$(CMD_PREFIX) kubectl rollout restart deploy/apiserver -n nexodus
	$(CMD_PREFIX) kubectl rollout restart deploy/frontend -n nexodus
	$(CMD_PREFIX) kubectl rollout restart deploy/apiproxy -n nexodus

.PHONY: init-db
init-db:
# wait for the DB to be up, then restart the services that use it.
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl wait -n nexodus postgresclusters/database --timeout=15m --for=condition=PGBackRestReplicaRepoReady || true
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl wait -n nexodus statefulsets/postgres --timeout=15m --for=jsonpath='{.status.readyReplicas}'=1 || true
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) make deploy-cockroach-operator
	$(CMD_PREFIX) kubectl -n nexodus wait --for=condition=Initialized crdbcluster/cockroachdb --timeout=15m
	$(CMD_PREFIX) kubectl -n nexodus rollout status statefulsets/cockroachdb --timeout=15m
	$(CMD_PREFIX) kubectl -n nexodus exec -it cockroachdb-0 \
	  	-- ./cockroach sql \
		--insecure \
		--certs-dir=/cockroach/cockroach-certs \
		--host=cockroachdb-public \
		--execute "\
			CREATE DATABASE IF NOT EXISTS ipam;\
			CREATE USER IF NOT EXISTS ipam;\
			GRANT ALL ON DATABASE ipam TO ipam;\
			CREATE DATABASE IF NOT EXISTS apiserver;\
			CREATE USER IF NOT EXISTS apiserver;\
			GRANT ALL ON DATABASE apiserver TO apiserver;\
			CREATE DATABASE IF NOT EXISTS keycloak;\
			CREATE USER IF NOT EXISTS keycloak;\
			GRANT ALL ON DATABASE keycloak TO keycloak;\
			"
endif
	$(CMD_PREFIX) kubectl rollout restart deploy/auth -n nexodus
	$(CMD_PREFIX) kubectl rollout restart deploy/apiserver -n nexodus
	$(CMD_PREFIX) kubectl rollout restart deploy/ipam -n nexodus
	$(CMD_PREFIX) kubectl -n nexodus rollout status deploy/auth --timeout=5m
	$(CMD_PREFIX) kubectl -n nexodus rollout status deploy/apiserver --timeout=5m
	$(CMD_PREFIX) kubectl -n nexodus rollout status deploy/ipam --timeout=5m

.PHONY: recreate-db
recreate-db: ## Delete and bring up a new nexodus database

	$(CMD_PREFIX) kubectl delete -n nexodus postgrescluster/database 2> /dev/null || true
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus postgrescluster/database
	$(CMD_PREFIX) kubectl delete -n nexodus statefulsets/postgres persistentvolumeclaims/postgres-disk-postgres-0 2> /dev/null || true
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus persistentvolumeclaims/postgres-disk-postgres-0
	$(CMD_PREFIX) kubectl delete -n nexodus crdbclusters/cockroachdb 2> /dev/null || true
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus --all pods -l app.kubernetes.io/name=cockroachdb --timeout=2m
	$(CMD_PREFIX) kubectl delete -n nexodus persistentvolumeclaims/datadir-cockroachdb-0 persistentvolumeclaims/datadir-cockroachdb-1 persistentvolumeclaims/datadir-cockroachdb-2 2> /dev/null || true
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus persistentvolumeclaims/datadir-cockroachdb-0
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus persistentvolumeclaims/datadir-cockroachdb-1
	$(CMD_PREFIX) kubectl wait --for=delete -n nexodus persistentvolumeclaims/datadir-cockroachdb-2

	$(CMD_PREFIX) kubectl apply -k ./deploy/nexodus/overlays/$(OVERLAY) | grep -v unchanged
	$(CMD_PREFIX) OVERLAY=$(OVERLAY) make init-db
	$(CMD_PREFIX) kubectl wait --for=condition=Ready pods --all -n nexodus -l app.kubernetes.io/part-of=nexodus --timeout=15m

.PHONY: cacerts
cacerts: ## Install the Self-Signed CA Certificate
	$(CMD_PREFIX) mkdir -p $(CURDIR)/.certs
	$(CMD_PREFIX) kubectl get secret -n nexodus nexodus-ca-key-pair -o json | jq -r '.data."ca.crt"' | base64 -d > $(CURDIR)/.certs/rootCA.pem
	$(CMD_PREFIX) CAROOT=$(CURDIR)/.certs mkcert -install

##@ Packaging

dist/rpm:
	$(CMD_PREFIX) mkdir -p dist/rpm

MOCK_ROOTS:=fedora-37-x86_64 fedora-38-x86_64
MOCK_DEPS:=golang systemd-rpm-macros systemd-units

.PHONY: image-mock
image-mock: ## Build and publish updated mock images to quay.io used for building rpms
	docker build -f Containerfile.mock -t quay.io/nexodus/mock:base .
	docker rm -f mock-base
	for MOCK_ROOT in $(MOCK_ROOTS) ; do \
		docker run --rm --name mock-base --privileged=true -d quay.io/nexodus/mock:base sleep 1800 ; \
		echo "Building mock root for $$MOCK_ROOT" ; \
		docker exec -it mock-base mock -r $$MOCK_ROOT --init ; \
		for MOCK_DEP in $(MOCK_DEPS) ; do \
			echo "Installing $$MOCK_DEP into $$MOCK_ROOT" ; \
			docker exec -it mock-base mock -r $$MOCK_ROOT --no-clean --no-cleanup-after --install $$MOCK_DEP ; \
		done ; \
		docker commit mock-base quay.io/nexodus/mock:$$MOCK_ROOT ; \
		docker rm -f mock-base ; \
		docker push quay.io/nexodus/mock:$$MOCK_ROOT ; \
	done

MOCK_ROOT?=fedora-37-x86_64
SRPM_DISTRO?=fc37

.PHONY: srpm
srpm: dist/rpm manpages ## Build a source RPM
	go mod vendor
	rm -rf dist/rpm/nexodus-${NEXODUS_RELEASE}
	rm -f dist/rpm/nexodus-${NEXODUS_RELEASE}.tar.gz
	git archive --format=tar.gz -o dist/rpm/nexodus-${NEXODUS_RELEASE}.tar.gz --prefix=nexodus-${NEXODUS_RELEASE}/ ${NEXODUS_RELEASE}
	cd dist/rpm && tar xzf nexodus-${NEXODUS_RELEASE}.tar.gz
	mv vendor dist/rpm/nexodus-${NEXODUS_RELEASE}/.
	mkdir -p dist/rpm/nexodus-${NEXODUS_RELEASE}/contrib/man
	cp -r contrib/man/* dist/rpm/nexodus-${NEXODUS_RELEASE}/contrib/man/.
	cd dist/rpm && tar czf nexodus-${NEXODUS_RELEASE}.tar.gz nexodus-${NEXODUS_RELEASE} && rm -rf nexodus-${NEXODUS_RELEASE}
	cp contrib/rpm/nexodus.spec.in contrib/rpm/nexodus.spec
	sed -i -e "s/##NEXODUS_COMMIT##/${NEXODUS_RELEASE}/" contrib/rpm/nexodus.spec
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:${MOCK_ROOT} \
		mock --buildsrpm -D "_commit ${NEXODUS_RELEASE}" --resultdir=/nexodus/dist/rpm/mock --no-clean --no-cleanup-after \
		--spec /nexodus/contrib/rpm/nexodus.spec --sources /nexodus/dist/rpm/ --root ${MOCK_ROOT}
	rm -f dist/rpm/nexodus-${NEXODUS_RELEASE}.tar.gz

.PHONY: rpm
rpm: srpm ## Build an RPM
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:${MOCK_ROOT} \
		mock --rebuild --without check --resultdir=/nexodus/dist/rpm/mock --root ${MOCK_ROOT} --no-clean --no-cleanup-after \
		/nexodus/$(wildcard dist/rpm/mock/nexodus-0-0.1.$(shell date --utc +%Y%m%d)git$(NEXODUS_RELEASE).$(SRPM_DISTRO).src.rpm)

##@ Documentation

contrib/man:
	$(CMD_PREFIX) mkdir -p contrib/man

.PHONY: manpages
manpages: contrib/man dist/nexd dist/nexctl ## Generate manpages in ./contrib/man
	dist/nexd -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:latest txt2man -t nexd | gzip > contrib/man/nexd.8.gz
	dist/nexctl -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:latest txt2man -t nexctl | gzip > contrib/man/nexctl.8.gz

# Nothing to see here
.PHONY: cat
cat:
	$(CMD_PREFIX) docker run -it --rm --name nyancat 06kellyjac/nyancat

.PHONY: docs
docs: ## Generate docs site into site/ directory
	$(CMD_PREFIX) docker run --rm -it -v ${PWD}:/docs squidfunk/mkdocs-material build

.PHONY: docs-preview
docs-preview: ## Generate a live preview of project documentation
	$(CMD_PREFIX) docker run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material


.PHONY: graph-prereqs
graph-prereqs:
	$(CMD_PREFIX) if ! which godepgraph >/dev/null 2>&1; then \
		echo "Please install godepgraph:" ; \
		echo "   go install github.com/kisielk/godepgraph@latest"; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) if ! which dot >/dev/null 2>&1; then \
		echo "Please install Graphviz." ; \
		echo "See: https://graphviz.org/download/" ; \
		exit 1 ; \
	fi

.PHONY: graph-all
graph-all: graph-nexd graph-nexctl graph-apiserver ## Graph the package dependencies of the binaries
graph-%: graph-prereqs
	@mkdir dist || true
	$(ECHO_PREFIX) printf "  %-12s dist/graph-$(word 2,$(subst -, ,$@)).png\n" "[GRAPHING]"
	$(CMD_PREFIX) godepgraph -s ./cmd/$(word 2,$(subst -, ,$@)) | dot -Tpng -o dist/graph-$(word 2,$(subst -, ,$@)).png
