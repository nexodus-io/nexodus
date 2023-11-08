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
    GOTESTSUM_FMT=standard-quiet
else
    ECHO_PREFIX=@\#
    CMD_PREFIX=
    PIPE_DEV_NULL=
	SWAG_ARGS?=
    GOTESTSUM_FMT=standard-verbose
endif

NEXODUS_VERSION?=$(shell date +%Y.%m.%d)
NEXODUS_RELEASE?=$(shell git describe --always --abbrev=6 --exclude qa --exclude prod)
NEXODUS_GCFLAGS?=

NEXODUS_KUBE_CONTEXT?=kind-nexodus-dev
NEXODUS_KUBE_NAMESPACE?=nexodus
kubectl:=kubectl --context=$(NEXODUS_KUBE_CONTEXT) --namespace=$(NEXODUS_KUBE_NAMESPACE)

NEXODUS_BUILD_PROFILE?=dev
NEXODUS_LDFLAGS:=$(NEXODUS_LDFLAGS) -X main.Version=$(NEXODUS_VERSION)-$(NEXODUS_RELEASE)
ifeq ($(NEXODUS_BUILD_PROFILE),dev)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://try.nexodus.127.0.0.1.nip.io
else ifeq ($(NEXODUS_BUILD_PROFILE),qa)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://qa.nexodus.io
else ifeq ($(NEXODUS_BUILD_PROFILE),prod)
	NEXODUS_LDFLAGS+=-X main.DefaultServiceURL=https://try.nexodus.io
endif

CGO_ENABLED?=0
ifeq ($(NEXODUS_RACE_DETECTOR),1)
    CGO_ENABLED=1
    NEXODUS_BUILD_FLAGS+=-race
endif
ifneq ($(NEXODUS_PPROF),)
    NEXODUS_BUILD_FLAGS+=-tags pprof
endif
ifneq ($(NEXODUS_BUILD_TAGS),)
	NEXODUS_BUILD_FLAGS+=-tags $(NEXODUS_BUILD_TAGS)
endif
ifeq ($(CGO_ENABLED),0)
    NEXODUS_LDFLAGS+=-extldflags=-static
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
all: gen-openapi-client generate go-lint yaml-lint md-lint ui-lint opa-lint action-lint apiserver nexd nexctl ## Run linters and build nexd

##@ Binaries

.PHONY: apiserver
apiserver: dist/apiserver dist/apiserver-pprof

.PHONY: nexd
nexd: dist/nexd dist/nexd-pprof dist/nexd-linux-arm dist/nexd-linux-arm64 dist/nexd-linux-amd64 dist/nexd-darwin-amd64 dist/nexd-darwin-arm64 dist/nexd-windows-amd64.exe ## Build the nexd binary for all architectures

.PHONY: nexd-kstore
nexd-kstore: dist/nexd-kstore ## Build the nexd-kstore binary

.PHONY: nexctl
nexctl: dist/nexctl dist/nexctl-linux-arm dist/nexctl-linux-arm64 dist/nexctl-linux-amd64 dist/nexctl-darwin-amd64 dist/nexctl-darwin-arm64 dist/nexctl-windows-amd64.exe ## Build the nexctl binary for all architectures

# Use go list to find all the go files that make up a binary.
NEXD_DEPS:=       $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/nexd)
NEXD_KSTORE_DEPS:=$(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/nexd-kstore)
NEXCTL_DEPS:=     $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/nexctl)
APISERVER_DEPS:=  $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./cmd/apiserver)
NEX_ALL_GO:=      $(shell go list -deps -f '{{if (and .Module (eq .Module.Path "github.com/nexodus-io/nexodus"))}}{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}{{end}}' ./...)

TAG=$(shell git rev-parse HEAD)

dist:
	$(CMD_PREFIX) mkdir -p $@

dist/apiserver: $(APISERVER_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) go build $(NEXODUS_BUILD_FLAGS) \
		-gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/apiserver

dist/apiserver-pprof: $(APISERVER_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) go build $(NEXODUS_BUILD_FLAGS) -tags pprof \
		-gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/apiserver

dist/nexd: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) go build $(NEXODUS_BUILD_FLAGS) \
		-gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexd-pprof: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) go build $(NEXODUS_BUILD_FLAGS) -tags pprof \
		-gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) go build $(NEXODUS_BUILD_FLAGS) -gcflags="$(NEXODUS_GCFLAGS)" \
		-ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexctl

dist/nexd-%: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build $(NEXODUS_BUILD_FLAGS) -gcflags="$(NEXODUS_GCFLAGS)" \
		-ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl-%: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build $(NEXODUS_BUILD_FLAGS) -gcflags="$(NEXODUS_GCFLAGS)" \
		-ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexctl

dist/nexd-kstore: $(NEXD_KSTORE_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) \
		go build -tags kubernetes $(NEXODUS_BUILD_FLAGS) -gcflags="$(NEXODUS_GCFLAGS)" \
		-ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd-kstore

dist/packages: \
	dist/packages/nexodus-linux-amd64.tar.gz \
	dist/packages/nexodus-linux-amd64.tar.gz \
	dist/packages/nexodus-linux-arm.tar.gz \
	dist/packages/nexodus-linux-arm64.tar.gz \
	dist/packages/nexodus-darwin-amd64.tar.gz \
	dist/packages/nexodus-darwin-arm64.tar.gz \
	dist/packages/nexodus-windows-amd64.zip

dist/packages/%: nexd nexctl $(shell find docs/user-guide/ -iname '*.md')
	$(CMD_PREFIX) mkdir -p $(basename $(basename $@))
	$(CMD_PREFIX) cp -r docs/user-guide $(basename $(basename $@))/user-guide
	$(CMD_PREFIX) cp LICENSE $(basename $(basename $@))
	$(CMD_PREFIX) cp README.md $(basename $(basename $@))
	$(CMD_PREFIX) cp contrib/bash_autocomplete $(basename $(basename $@))
	$(CMD_PREFIX) cp dist/nexd-$(subst nexodus-,,$(basename $(basename $(@F))))$(if $(findstring windows,$@),.exe) $(basename $(basename $@))/nexd$(if $(findstring windows,$@),.exe)
	$(CMD_PREFIX) cp dist/nexctl-$(subst nexodus-,,$(basename $(basename $(@F))))$(if $(findstring windows,$@),.exe) $(basename $(basename $@))/nexctl$(if $(findstring windows,$@),.exe)
	$(CMD_PREFIX) if test "$(word 2,$(subst -, ,$(shell basename $@)))" = "windows" ; then \
		printf "  %-12s dist/packages/$(@F)\n" "[ZIP]" ;\
		cd dist/packages && zip -q9r $(@F) $(basename $(basename $(@F))) ;\
	else \
		printf "  %-12s dist/packages/$(@F)\n" "[TAR]" ;\
		cd dist/packages && tar -czf $(@F) $(basename $(basename $(@F)))  ;\
	fi

.PHONY: clean
clean: ## clean built binaries
	$(CMD_PREFIX) touch ./cmd/apiserver/main.go # to force apidocs to get rebuilt.
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
	$(CMD_PREFIX) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(word 3,$(subst -, ,$@)) GOARCH=amd64 \
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


.PHONY: k8s-lint
k8s-lint: dist/.k8s-lint ## Lint the kubernetes deployment files

dist/.k8s-lint: $(shell find deploy -type f ) | dist
	$(CMD_PREFIX) if ! which kubeconform >/dev/null 2>&1; then \
		echo "Please install kubeconform." ; \
		echo "With: go install github.com/yannh/kubeconform/cmd/kubeconform@v0.5.0" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) mkdir -p ./dist/kubeconfigs
	$(CMD_PREFIX) kustomize build ./deploy/nexodus/overlays/dev > ./dist/kubeconfigs/dev.yaml
	$(CMD_PREFIX) kustomize build ./deploy/nexodus/overlays/prod > ./dist/kubeconfigs/prod.yaml
	$(CMD_PREFIX) kustomize build ./deploy/nexodus/overlays/qa > ./dist/kubeconfigs/qa.yaml
	$(CMD_PREFIX) kustomize build ./deploy/nexodus/overlays/playground > ./dist/kubeconfigs/playground.yaml
	$(CMD_PREFIX) kubeconform -summary -output json -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' -schema-location 'deploy/.crdSchemas/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' ./dist/kubeconfigs/
	$(CMD_PREFIX) touch $@

dist/crd-extractor.zip: dist
	$(CMD_PREFIX) if [ ! -f $@  ] ; then \
	   curl -L -s https://github.com/datreeio/CRDs-catalog/releases/latest/download/crd-extractor.zip --output dist/crd-extractor.zip; \
	fi

dist/crd-extractor.sh: dist/crd-extractor.zip
	$(CMD_PREFIX) cd dist && unzip -o crd-extractor.zip
	$(CMD_PREFIX) chmod a+x dist/crd-extractor.sh
	$(CMD_PREFIX) touch $@

.PHONY: k8s-crd-extract
k8s-crd-extract: dist/crd-extractor.sh ## Extract the kubernetes CRDs used iin k8s-lint
	$(CMD_PREFIX) rm -rf $(HOME)/.datree/crdSchemas
	$(CMD_PREFIX) dist/crd-extractor.sh
	$(CMD_PREFIX) cp $(HOME)/.datree/crdSchemas/*/* deploy/.crdSchemas


.PHONY: png-lint
png-lint: dist/.png-lint ## Lint the png files from excalidraw

dist/.png-lint: $(shell find . -iname '*.png') | dist
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[PNG LINT]"
	$(CMD_PREFIX) for file in $^; do \
		if grep -q "$$file" .excalidraw-ignore; then continue ; fi ; \
		if ! grep -q "excalidraw+json" $$file; then \
			echo "$$file was not exported from excalidraw with 'Embed Scene' enabled." ; \
			echo "If this is not an excalidraw file, add it to .excalidraw-ignore" ; \
			exit 1 ; \
		fi \
	done
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

.PHONY: action-lint
action-lint: dist/.action-lint ## Lint GitHub Action workflows
dist/.action-lint: $(find -type f .github) | dist
	$(ECHO_PREFIX) printf "  %-12s .github/...\n" "[ACTION LINT]"
	$(CMD_PREFIX) if ! which actionlint $(PIPE_DEV_NULL) ; then \
		echo "Please install actionlint." ; \
		echo "go install github.com/rhysd/actionlint/cmd/actionlint@latest" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) if ! which shellcheck $(PIPE_DEV_NULL) ; then \
		echo "Please install shellcheck." ; \
		echo "https://github.com/koalaman/shellcheck#user-content-installing" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) actionlint -color
	$(CMD_PREFIX) touch $@

# https://github.com/google/go-licenses
# For license IDs: https://github.com/google/licenseclassifier/blob/main/license_type.go
DISALLOWED_LICENSE_TYPES:=forbidden,restricted,unknown
.PHONY: go-licenses
go-licenses: dist/.go-licenses ## Validate licenses of Go dependencies
dist/.go-licenses: $(NEX_ALL_GO) | dist
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO LICENSES]"
	$(CMD_PREFIX) if ! which go-licenses $(PIPE_DEV_NULL) ; then \
		echo "Please install go-licenses." ; \
		echo "go install github.com/google/go-licenses@latest" ; \
		exit 1 ; \
	fi
	$(CMD_PREFIX) go-licenses check --include_tests --disallowed_types=$(DISALLOWED_LICENSE_TYPES) ./...
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
		ghcr.io/swaggo/swag:v1.16.1 \
		/root/swag init $(SWAG_ARGS) -g ./cmd/apiserver/main.go -o ./internal/docs

.PHONY: gen-openapi-client
gen-openapi-client: internal/api/public/client.go ## Generate the OpenAPI Client
internal/api/public/client.go: internal/docs/swagger.yaml | dist
	$(ECHO_PREFIX) printf "  %-12s internal/docs/swagger.yaml\n" "[OPENAPI CLIENT GEN]"
	$(CMD_PREFIX) rm -f $(shell find internal/api/public | grep .go | grep -v _custom.go)
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/src --user $(shell id -u):$(shell id -g) \
		openapitools/openapi-generator-cli:v6.5.0 \
		generate -i /src/internal/docs/swagger.yaml -g go \
		--package-name public \
		-o /src/internal/api/public \
		-t /src/hack/openapi-templates \
		--ignore-file-override /src/.openapi-generator-ignore $(PIPE_DEV_NULL)
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO FMT]"
	$(CMD_PREFIX) [ -z "$$(gofmt -l .)" ] || gofmt -w .

internal/api/public/%.go: internal/api/public/client.go

.PHONY: opa-fmt
opa-fmt: ## Lint the OPA policies
	$(ECHO_PREFIX) printf "  %-12s \n" "[OPA FMT]"
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest fmt --write $(policies)


.PHONY: ui-fmt
ui-fmt: dist/.ui-fmt ## Format the UI sources
dist/.ui-fmt: $(wildcard ui/*) $(wildcard ui/src/**) | dist
	$(ECHO_PREFIX) printf "  %-12s \n" "[UI FMT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir --user $(shell id -u):$(shell id -g) \
		tmknom/prettier --write /workdir/ui/src/ $(PIPE_DEV_NULL)
	$(CMD_PREFIX) touch $@

.PHONY: generate
generate: dist/.generate ## Run all code generators and formatters

docs/user-guide/nexd.md: dist/nexd hack/nexd-docs.sh
	$(ECHO_PREFIX) printf "  %-12s nexd\n" "[DOCS]"
	$(CMD_PREFIX) hack/nexd-docs.sh

docs/user-guide/nexctl.md: dist/nexctl hack/nexctl-docs.sh
	$(ECHO_PREFIX) printf "  %-12s nexctl\n" "[DOCS]"
	$(CMD_PREFIX) hack/nexctl-docs.sh

dist/.generate: $(SWAGGER_YAML) dist/.ui-fmt docs/user-guide/nexd.md docs/user-guide/nexctl.md | dist
	$(ECHO_PREFIX) printf "  %-12s \n" "[MOD TIDY]"
	$(CMD_PREFIX) go mod tidy

	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO FMT]"
	$(CMD_PREFIX) [ -z "$(shell gofmt -l .)" ] || gofmt -w .
	$(CMD_PREFIX) touch $@

.PHONY: e2e
e2e: e2eprereqs dist/nexd dist/nexctl image-nexd ## Run e2e verbose tests
	CGO_ENABLED=1 gotestsum --format $(GOTESTSUM_FMT) -- \
		-race --tags=integration ./integration-tests/... $(shell [ -z "$$NEX_TEST" ] || echo "-run $$NEX_TEST" )

.PHONY: e2e-podman
e2e-podman: ## Run e2e tests on podman
	go test -c -v --tags=integration ./integration-tests/...
	sudo NEXODUS_TEST_PODMAN=1 TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED=true ./integration-tests.test -test.v

.PHONY: test
test: gotestsum-prereqs ## Run unit tests
	gotestsum --format $(GOTESTSUM_FMT) -- \
		./...

.PHONY: telepresence-connect-f
telepresence-connect-f: telepresence-prereqs
	telepresence helm install 2> /dev/null || true ;\
	telepresence quit -s || true ;\
	telepresence connect --context=$(NEXODUS_KUBE_CONTEXT) --namespace=$(NEXODUS_KUBE_NAMESPACE);\

.PHONY: telepresence-connect
telepresence-connect: telepresence-prereqs
	$(CMD_PREFIX) if [ "$(shell telepresence status --output json | jq .user_daemon.status -r)" != "Connected" ]; then \
		make telepresence-connect-f ;\
	fi

telepresence_%: telepresence-connect
	$(CMD_PREFIX) if [ -z "$(shell telepresence status --output json | jq '.user_daemon.intercepts[]|select(.name == "$(word 2,$(subst _, ,$(basename $@)))-nexodus")' 2> /dev/null)" ]; then \
		telepresence intercept $(word 2,$(subst _, ,$(basename $@))) --port $(word 3,$(subst _, ,$(basename $@))) --env-json=$(word 2,$(subst _, ,$(basename $@)))-envs.json ;\
		echo "=======================================================================================" ;\
		echo ;\
		echo "   Start the $(word 2,$(subst _, ,$(basename $@))) locally with a debugger with the env variables" ;\
		echo "   with the values found in: $(word 2,$(subst _, ,$(basename $@)))-envs.json" ;\
		echo ;\
		echo "   Hint: use the IDEA EnvFile plugin https://plugins.jetbrains.com/plugin/7861-envfile" ;\
		echo ;\
	fi

.PHONY: debug-apiserver
debug-apiserver: telepresence-connect ## Use telepresence to debug the apiserver deployment
	$(CMD_PREFIX) if [ -z "$(shell telepresence status --output json | jq '.user_daemon.intercepts[]|select(.name == "$(word 2,$(subst _, ,$(basename $@)))-nexodus")' 2> /dev/null)" ]; then \
		telepresence intercept apiserver --workload apiserver --service apiserver --port 8080:8080 --env-json=apiserver-envs.json ;\
		telepresence intercept apiserver-grpc --workload apiserver --service apiserver --port 5080:5080 ;\
		echo "=======================================================================================" ;\
		echo ;\
		echo "   Start the apiserver locally with a debugger with the env variables" ;\
		echo "   with the values found in: apiserver-envs.json" ;\
		echo ;\
		echo "   Hint: use the IDEA EnvFile plugin https://plugins.jetbrains.com/plugin/7861-envfile" ;\
		echo ;\
	fi


.PHONY: debug-apiserver-stop
debug-apiserver-stop: telepresence-prereqs ## Stop using telepresence to debug the apiserver deployment
	$(CMD_PREFIX) telepresence leave apiserver
	$(CMD_PREFIX) telepresence leave apiserver-grpc

dist/.npm-install:
	$(CMD_PREFIX) cd ui; npm install
	$(CMD_PREFIX) touch $@

.PHONY: debug-frontend
debug-frontend: telepresence_frontend_3000 dist/.npm-install ## Use telepresence to debug the frontend deployment
	$(CMD_PREFIX) cd ui; npm start

.PHONY: debug-frontend-stop
debug-frontend-stop: telepresence-prereqs ## Stop using telepresence to debug the frontend deployment
	$(CMD_PREFIX) telepresence leave frontend

NEXODUS_LOCAL_IP:=`go run ./hack/localip`
.PHONY: run-nexd-container
run-nexd-container: image-nexd ## Run a container that you can run nexodus in
	$(CMD_PREFIX) mkdir -p .certs
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
	$(CMD_PREFIX) $(kubectl) exec -it \
		$(shell $(kubectl) get pods -l postgres-operator.crunchydata.com/role=master -o name) \
		-c database -- psql apiserver
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) $(kubectl) exec -it svc/postgres -c postgres -- psql -U apiserver apiserver
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) $(kubectl) exec -it svc/cockroachdb -- cockroach sql --insecure --user apiserver --database apiserver
endif

.PHONY: run-sql-ipam
run-sql-ipam: ## runs a command line SQL client to interact with the ipam database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) $(kubectl) exec -it \
		$(shell $(kubectl) get pods -l postgres-operator.crunchydata.com/role=master -o name) \
		-c database -- psql ipam
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) $(kubectl) exec -it svc/postgres -c postgres -- psql -U ipam ipam
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) $(kubectl) exec -it svc/cockroachdb -- cockroach sql --insecure --user ipam --database ipam
endif

.PHONY: run-sql-keycloak
run-sql-keycloak: ## runs a command line SQL client to interact with the keycloak database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) $(kubectl) exec -it \
		$(shell $(kubectl) get pods -l postgres-operator.crunchydata.com/role=master -o name) \
		-c database -- psql keycloak
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) $(kubectl) exec -it svc/postgres -c postgres -- psql -U keycloak keycloak
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) $(kubectl) exec -it svc/cockroachdb -- cockroach sql --insecure --user keycloak --database keycloak
endif


.PHONY: clear-db
clear-db:
	$(CMD_PREFIX) $(kubectl) scale deployment apiserver --replicas=0 $(PIPE_DEV_NULL)
	$(CMD_PREFIX) $(kubectl) rollout status deploy/apiserver --timeout=5m $(PIPE_DEV_NULL)
	$(ECHO_PREFIX) printf "  %-12s \n" "[DROP TABLES] ..."
	$(CMD_PREFIX) echo "\
		DROP TABLE IF EXISTS reg_keys;\
		DROP TABLE IF EXISTS registration_tokens;\
		DROP TABLE IF EXISTS invitations;\
		DROP TABLE IF EXISTS security_groups;\
		DROP TABLE IF EXISTS device_metadata;\
		DROP TABLE IF EXISTS devices;\
		DROP TABLE IF EXISTS user_organizations;\
		DROP TABLE IF EXISTS vpcs;\
		DROP TABLE IF EXISTS organizations;\
		DROP TABLE IF EXISTS users;\
		DROP TABLE IF EXISTS apiserver_migrations;\
		" | make run-sql-apiserver $(PIPE_DEV_NULL)
	$(CMD_PREFIX) $(kubectl) scale deployment apiserver --replicas=1 $(PIPE_DEV_NULL)
	$(CMD_PREFIX) $(kubectl) rollout status deploy/apiserver --timeout=5m
	$(CMD_PREFIX) $(kubectl) rollout restart statefulset redis $(PIPE_DEV_NULL)
	$(CMD_PREFIX) $(kubectl) rollout status statefulset redis --timeout=5m

##@ Container Images

.PHONY: e2eprereqs
e2eprereqs: gotestsum-prereqs
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

.PHONY: gotestsum-prereqs
gotestsum-prereqs:
	$(CMD_PREFIX) if [ -z "$(shell which gotestsum)" ]; then \
		echo "Please install gotestsum." ; \
		echo "  $$ ./hack/install-tools.sh" ; \
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
	docker build -f Containerfile.apiserver \
		--build-arg NEXODUS_PPROF="$(NEXODUS_PPROF)" \
		--build-arg NEXODUS_RACE_DETECTOR="$(NEXODUS_RACE_DETECTOR)" \
		-t quay.io/nexodus/apiserver:$(TAG) .
	docker tag quay.io/nexodus/apiserver:$(TAG) quay.io/nexodus/apiserver:latest

.PHONY: image-nexd ## Build the nexodus agent image
image-nexd: dist/.image-nexd
dist/.image-nexd: $(NEXD_DEPS) $(NEXCTL_DEPS) Containerfile.nexd hack/update-ca.sh | dist
	$(CMD_PREFIX) docker build -f Containerfile.nexd \
		--build-arg NEXODUS_PPROF="$(NEXODUS_PPROF)" \
		--build-arg NEXODUS_RACE_DETECTOR="$(NEXODUS_RACE_DETECTOR)" \
		-t quay.io/nexodus/nexd:$(TAG) .
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
images: image-nexd image-frontend image-apiserver image-ipam image-envsubst ## Create container images

##@ Kubernetes - kind dev environment

.PHONY: run-on-kind
run-on-kind: setup-kind install-olm deploy-operators images load-images deploy cacerts ## Setup a kind cluster and deploy nexodus on it

.PHONY: teardown
teardown: ## Teardown the kind cluster
	$(CMD_PREFIX) kind delete cluster --name nexodus-dev

.PHONY: setup-kind
setup-kind: teardown ## Create a kind cluster with ingress enabled, but don't install nexodus.
	$(CMD_PREFIX) kind create cluster --config ./deploy/kind.yaml
	$(CMD_PREFIX) $(kubectl) cluster-info
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -f ./deploy/kind-ingress.yaml

.PHONY: deploy-nexodus-agent ## Deply the nexodus agent in the kind cluster
deploy-nexodus-agent: image-nexd
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/nexd:latest
	$(CMD_PREFIX) cp deploy/nexodus-client/overlays/dev/kustomization.yaml.sample deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) sed -i -e "s/<NEXODUS_SERVICE_IP>/$(NEXODUS_LOCAL_IP)/" deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) sed -i -e "s/<NEXODUS_SERVICE_CERT>/$(shell $(kubectl) get secret nexodus-ca-key-pair -o json | jq -r '.data."ca.crt"')/" deploy/nexodus-client/overlays/dev/kustomization.yaml
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -k ./deploy/nexodus-client/overlays/dev

##@ Kubernetes - work with an existing cluster (kind dev env or another one)
.PHONY: install-olm
install-olm: ## Install OLM on the cluster
	$(CMD_PREFIX) curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.24.0/install.sh | bash -s v0.24.0

.PHONY: deploy-operators
deploy-operators: ## Deploy all operators and wait for readiness
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -k ./deploy/operators/overlays/$(OVERLAY)

.PHONY: use-cockroach
use-cockroach: ## Recreate the database with a Cockroach based server
	$(CMD_PREFIX) OVERLAY=cockroach make recreate-db

.PHONY: use-crunchy
use-crunchy: ## Recreate the database with a Crunchy based postgres server
	$(CMD_PREFIX) OVERLAY=dev make recreate-db

.PHONY: use-postgres
use-postgres: ## Recreate the database with a simple Postgres server
	$(CMD_PREFIX) OVERLAY=arm64 make recreate-db

.PHONY: wait-for-readiness
wait-for-readiness: # Wait for operators to be ready
	$(CMD_PREFIX) ./hack/wait-for-labelled-resource.sh csv --context=$(NEXODUS_KUBE_CONTEXT) -n operators -l operators.coreos.com/cert-manager.operators
	$(CMD_PREFIX) ./hack/wait-for-labelled-resource.sh csv --context=$(NEXODUS_KUBE_CONTEXT) -n operators -l operators.coreos.com/prometheus.operators
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) -n operators wait --for=jsonpath='{.status.phase}'=Succeeded csv --all --timeout=5m
	$(CMD_PREFIX) ./hack/wait-for-labelled-resource.sh csv --context=$(NEXODUS_KUBE_CONTEXT) -n nexodus-monitoring -l operators.coreos.com/grafana-operator.nexodus-monitoring
	$(CMD_PREFIX) kubectl wait --context=$(NEXODUS_KUBE_CONTEXT) -n nexodus-monitoring --for=jsonpath='{.status.phase}'=Succeeded csv --all --timeout=5m
	$(CMD_PREFIX) ./hack/wait-for-resource-exists.sh --context=$(NEXODUS_KUBE_CONTEXT) -n ingress-nginx secrets ingress-nginx-admission
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) -n ingress-nginx rollout restart deployment ingress-nginx-controller
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) -n ingress-nginx rollout status deployment ingress-nginx-controller --timeout=5m

.PHONY: deploy
deploy: wait-for-readiness ## Deploy a development nexodus stack onto a kubernetes cluster
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) create namespace nexodus
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -k ./deploy/nexodus/overlays/$(OVERLAY)
	$(CMD_PREFIX) OVERLAY=$(OVERLAY) make init-db
	$(CMD_PREFIX) $(kubectl) wait --for=condition=Ready pods --all -l app.kubernetes.io/part-of=nexodus --timeout=15m

.PHONY: undeploy
undeploy: ## Remove the nexodus stack from a kubernetes cluster
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) delete namespace nexodus

.PHONY: deploy-monitoring-stack ## Deploy the monitoring stack in the kind cluster
deploy-monitoring-stack:
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -k ./deploy/nexodus-monitoring/overlays/dev
	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) -n nexodus-monitoring wait --for=condition=Ready pods --all --timeout=15m

.PHONY: load-images
load-images: ## Load images onto kind
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/apiserver:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/frontend:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/go-ipam:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/envsubst:latest
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image quay.io/nexodus/nexd:latest
	$(CMD_PREFIX) docker pull docker.io/library/redis:6.0
	$(CMD_PREFIX) kind load --name nexodus-dev docker-image docker.io/library/redis:6.0

.PHONY: redeploy
redeploy: images load-images ## Redeploy nexodus after images changes
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/apiserver
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/frontend
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/apiproxy

.PHONY: init-db
init-db:
# wait for the DB to be up, then restart the services that use it.
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) $(kubectl) wait postgresclusters/database --timeout=15m --for=condition=PGBackRestReplicaRepoReady || true
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) $(kubectl) wait statefulsets/postgres --timeout=15m --for=jsonpath='{.status.readyReplicas}'=1 || true
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) make deploy-cockroach-operator
	$(CMD_PREFIX) $(kubectl) wait --for=condition=Initialized crdbcluster/cockroachdb --timeout=15m
	$(CMD_PREFIX) $(kubectl) rollout status statefulsets/cockroachdb --timeout=15m
	$(CMD_PREFIX) $(kubectl) exec -it cockroachdb-0 \
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
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/auth
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/apiserver
	$(CMD_PREFIX) $(kubectl) rollout restart deploy/ipam
	$(CMD_PREFIX) $(kubectl) rollout status deploy/auth --timeout=5m
	$(CMD_PREFIX) $(kubectl) rollout status deploy/apiserver --timeout=5m
	$(CMD_PREFIX) $(kubectl) rollout status deploy/ipam --timeout=5m

.PHONY: recreate-db
recreate-db: ## Delete and bring up a new nexodus database
	$(CMD_PREFIX) $(kubectl) delete postgrescluster/database 2> /dev/null || true
	$(CMD_PREFIX) $(kubectl) wait --for=delete postgrescluster/database || true
	$(CMD_PREFIX) $(kubectl) delete statefulsets/postgres persistentvolumeclaims/postgres-disk-postgres-0 2> /dev/null || true
	$(CMD_PREFIX) $(kubectl) wait --for=delete persistentvolumeclaims/postgres-disk-postgres-0
	$(CMD_PREFIX) $(kubectl) delete crdbclusters/cockroachdb 2> /dev/null || true
	$(CMD_PREFIX) $(kubectl) wait --for=delete --all pods -l app.kubernetes.io/name=cockroachdb --timeout=2m
	$(CMD_PREFIX) $(kubectl) delete persistentvolumeclaims/datadir-cockroachdb-0 persistentvolumeclaims/datadir-cockroachdb-1 persistentvolumeclaims/datadir-cockroachdb-2 2> /dev/null || true
	$(CMD_PREFIX) $(kubectl) wait --for=delete persistentvolumeclaims/datadir-cockroachdb-0
	$(CMD_PREFIX) $(kubectl) wait --for=delete persistentvolumeclaims/datadir-cockroachdb-1
	$(CMD_PREFIX) $(kubectl) wait --for=delete persistentvolumeclaims/datadir-cockroachdb-2

	$(CMD_PREFIX) kubectl --context=$(NEXODUS_KUBE_CONTEXT) apply -k ./deploy/nexodus/overlays/$(OVERLAY) | grep -v unchanged
	$(CMD_PREFIX) OVERLAY=$(OVERLAY) make init-db
	$(CMD_PREFIX) $(kubectl) wait --for=condition=Ready pods --all -l app.kubernetes.io/part-of=nexodus --timeout=15m
	$(CMD_PREFIX) $(kubectl) rollout restart statefulset redis $(PIPE_DEV_NULL)
	$(CMD_PREFIX) $(kubectl) rollout status statefulset redis --timeout=5m

.PHONY: cacerts
cacerts: ## Install the Self-Signed CA Certificate
	$(CMD_PREFIX) mkdir -p $(CURDIR)/.certs
	$(CMD_PREFIX) $(kubectl) get secret nexodus-ca-key-pair -o json | jq -r '.data."ca.crt"' | base64 -d > $(CURDIR)/.certs/rootCA.pem
	$(CMD_PREFIX) CAROOT=$(CURDIR)/.certs mkcert -install

##@ Packaging

dist/rpm:
	$(CMD_PREFIX) mkdir -p dist/rpm

MOCK_ROOTS?=fedora-38-x86_64 centos-stream+epel-9-x86_64
MOCK_DEPS:=golang systemd-rpm-macros systemd-units
MOCK_CONTAINER_DEPS?=

.PHONY: image-mock
image-mock: ## Build and publish updated mock images to quay.io used for building rpms
	docker build -f Containerfile.mock -t quay.io/nexodus/mock:base .
	docker rm -f mock-base
	for MOCK_ROOT in $(MOCK_ROOTS) ; do \
		docker run --rm --name mock-base --privileged=true -d quay.io/nexodus/mock:base sleep 1800 ; \
		for MOCK_DEP in $(MOCK_CONTAINER_DEPS) ; do \
			echo "===== Installing $$MOCK_DEP into mock-base" ; \
			docker exec -it mock-base dnf install -y $$MOCK_DEP ; \
		done ; \
		echo "===== Building mock root for $$MOCK_ROOT" ; \
		docker exec -it mock-base mock -r $$MOCK_ROOT --init ; \
		for MOCK_DEP in $(MOCK_DEPS) ; do \
			echo "===== Installing $$MOCK_DEP into $$MOCK_ROOT" ; \
			docker exec -it mock-base mock -r $$MOCK_ROOT --no-clean --no-cleanup-after --install $$MOCK_DEP ; \
		done ; \
		docker commit mock-base quay.io/nexodus/mock:$$(echo $$MOCK_ROOT | cut -f2 -d'+') ; \
		docker rm -f mock-base ; \
		docker push quay.io/nexodus/mock:$$(echo $$MOCK_ROOT | cut -f2 -d'+') ; \
	done

MOCK_ROOTS_AARCH64:=fedora-38-aarch64 centos-stream+epel-9-aarch64

.PHONY: image-mock-aarch64
image-mock-aarch64: ## Build and publish updated mock images to quay.io used for building rpms
	MOCK_ROOTS="$(MOCK_ROOTS_AARCH64)" MOCK_CONTAINER_DEPS="qemu-user-static-aarch64" $(MAKE) image-mock

MOCK_ROOT?=fedora-38-x86_64
SRPM_MOCK_ROOT?=fedora-38-x86_64
SRPM_DISTRO?=fc38
NEXODUS_AUTORELEASE=0.1.$(shell date -u +%Y%m%d)git$(NEXODUS_RELEASE).$(SRPM_DISTRO)

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
	NEXODUS_BUILD_PROFILE=prod $(MAKE) ldflags.txt
	mv ldflags.txt dist/rpm/nexodus-${NEXODUS_RELEASE}/.
	cd dist/rpm && tar czf nexodus-${NEXODUS_RELEASE}.tar.gz nexodus-${NEXODUS_RELEASE} && rm -rf nexodus-${NEXODUS_RELEASE}
	cp contrib/rpm/nexodus.spec.in contrib/rpm/nexodus.spec
	sed -i -e "s/##NEXODUS_COMMIT##/${NEXODUS_RELEASE}/" contrib/rpm/nexodus.spec
	sed -i -e "s/##NEXODUS_AUTORELEASE##/$(NEXODUS_AUTORELEASE)/" contrib/rpm/nexodus.spec
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:$$(echo $(SRPM_MOCK_ROOT) | cut -f2 -d'+') \
		mock --buildsrpm -D "_commit ${NEXODUS_RELEASE}" --resultdir=/nexodus/dist/rpm/mock --no-clean --no-cleanup-after \
		--spec /nexodus/contrib/rpm/nexodus.spec --sources /nexodus/dist/rpm/ --root ${SRPM_MOCK_ROOT}
	rm -f dist/rpm/nexodus-${NEXODUS_RELEASE}.tar.gz

.PHONY: rpm
rpm: srpm ## Build an RPM
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:$$(echo $(MOCK_ROOT) | cut -f2 -d'+') \
		mock --rebuild --without check --resultdir=/nexodus/dist/rpm/mock --root ${MOCK_ROOT} \
		--no-clean --no-cleanup-after --enable-network \
		/nexodus/$(wildcard dist/rpm/mock/nexodus-0-0.1.$(shell date -u +%Y%m%d)git$(NEXODUS_RELEASE).$(SRPM_DISTRO).src.rpm)

.PHONY: version
version: ## Print the version string
	$(CMD_PREFIX) echo "${NEXODUS_VERSION}-${NEXODUS_RELEASE}"

ldflags.txt:
	@echo "$(NEXODUS_LDFLAGS) " > ldflags.txt

##@ Documentation

contrib/man:
	$(CMD_PREFIX) mkdir -p contrib/man

.PHONY: manpages
manpages: contrib/man dist/nexd dist/nexctl ## Generate manpages in ./contrib/man
	dist/nexd -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:$$(echo $(MOCK_ROOT) | cut -f2 -d'+') txt2man -t nexd | gzip > contrib/man/nexd.8.gz
	dist/nexctl -h | docker run -i --rm --name txt2man quay.io/nexodus/mock:$$(echo $(MOCK_ROOT) | cut -f2 -d'+') txt2man -t nexctl | gzip > contrib/man/nexctl.8.gz

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
