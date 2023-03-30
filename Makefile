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
NEXODUS_RELEASE?=$(shell git describe --always)
NEXODUS_LDFLAGS?=-X main.Version=$(NEXODUS_VERSION)-$(NEXODUS_RELEASE)
NEXODUS_GCFLAGS?=

# Crunchy DB operator does not work well on arm64, use an different overlay to work around it.
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),arm64)
	OVERLAY?=arm64
else
	OVERLAY?=dev
endif

##@ All

.PHONY: all
all: gen-docs go-lint yaml-lint md-lint ui-lint nexd nexctl ## Run linters and build nexd

##@ Binaries

.PHONY: nexd
nexd: dist/nexd dist/nexd-linux-arm dist/nexd-linux-amd64 dist/nexd-darwin-amd64 dist/nexd-darwin-arm64 dist/nexd-windows-amd64 ## Build the nexd binary for all architectures

.PHONY: nexctl
nexctl: dist/nexctl dist/nexctl-linux-arm dist/nexctl-linux-amd64 dist/nexctl-darwin-amd64 dist/nexctl-darwin-arm64 dist/nexctl-windows-amd64 ## Build the nexctl binary for all architectures

COMMON_DEPS=$(wildcard ./internal/**/*.go) go.sum go.mod

NEXD_DEPS=$(COMMON_DEPS) $(wildcard cmd/nexd/*.go)

NEXCTL_DEPS=$(COMMON_DEPS) $(wildcard cmd/nexctl/*.go)

APISERVER_DEPS=$(COMMON_DEPS) $(wildcard cmd/apiserver/*.go)

TAG=$(shell git rev-parse HEAD)

dist:
	$(CMD_PREFIX) mkdir -p $@

dist/nexd: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexctl

dist/nexd-%: $(NEXD_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexd

dist/nexctl-%: $(NEXCTL_DEPS) | dist
	$(ECHO_PREFIX) printf "  %-12s $@\n" "[GO BUILD]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 2,$(subst -, ,$(basename $@))) GOARCH=$(word 3,$(subst -, ,$(basename $@))) \
		go build -gcflags="$(NEXODUS_GCFLAGS)" -ldflags="$(NEXODUS_LDFLAGS)" -o $@ ./cmd/nexctl

.PHONY: clean
clean: ## clean built binaries
	rm -rf dist

##@ Development

.PHONY: go-lint
go-lint: go-lint-linux go-lint-darwin go-lint-windows ## Lint the go code

.PHONY: go-lint-prereqs
go-lint-prereqs:
	$(CMD_PREFIX) if ! which golangci-lint >/dev/null 2>&1; then \
		echo "Please install golangci-lint." ; \
		echo "See: https://golangci-lint.run/usage/install/#local-installation" ; \
		exit 1 ; \
	fi

.PHONY: go-lint-%
go-lint-%: go-lint-prereqs $(NEXD_DEPS) $(NEXCTL_DEPS) $(APISERVER_DEPS)
	$(ECHO_PREFIX) printf "  %-12s GOOS=$(word 3,$(subst -, ,$(basename $@)))\n" "[GO LINT]"
	$(CMD_PREFIX) CGO_ENABLED=0 GOOS=$(word 3,$(subst -, ,$(basename $@))) GOARCH=amd64 \
		golangci-lint run --timeout 5m ./...

.PHONY: yaml-lint
yaml-lint: ## Lint the yaml files
	$(CMD_PREFIX) if ! which yamllint >/dev/null 2>&1; then \
		echo "Please install yamllint." ; \
		echo "See: https://yamllint.readthedocs.io/en/stable/quickstart.html" ; \
		exit 1 ; \
	fi
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[YAML LINT]"
	$(CMD_PREFIX) yamllint -c .yamllint.yaml deploy --strict

.PHONY: md-lint
md-lint: ## Lint markdown files
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[MD LINT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir docker.io/davidanson/markdownlint-cli2:v0.6.0 > /dev/null

.PHONY: ui-lint
ui-lint: ## Lint the UI source
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[UI LINT]"
	$(CMD_PREFIX) docker run --rm -v $(CURDIR):/workdir tmknom/prettier --check /workdir/ui/src/ >/dev/null

policies=$(wildcard internal/routers/*.rego)

.PHONY: opa-lint
opa-lint: ## Lint the OPA policies
	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[OPA LINT]"
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest fmt --fail $(policies) $(PIPE_DEV_NULL)
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir docker.io/openpolicyagent/opa:latest test -v $(policies) $(PIPE_DEV_NULL)

.PHONY: gen-docs
gen-docs: ## Generate API docs
	$(ECHO_PREFIX) printf "  %-12s ./cmd/apiserver/main.go\n" "[API DOCS]"
	$(CMD_PREFIX) docker run --platform linux/x86_64 --rm -v $(CURDIR):/workdir -w /workdir ghcr.io/swaggo/swag:v1.8.10 /root/swag init $(SWAG_ARGS) --exclude pkg -g ./cmd/apiserver/main.go -o ./internal/docs

.PHONY: generate
generate: gen-docs ## Run all code generators and formatters
	$(ECHO_PREFIX) printf "  %-12s \n" "[MOD TIDY]"
	$(CMD_PREFIX) go mod tidy

	$(ECHO_PREFIX) printf "  %-12s ./...\n" "[GO FMT]"
	$(CMD_PREFIX) go fmt ./...

.PHONY: e2e
e2e: e2eprereqs test-images ## Run e2e tests
	go test -v --tags=integration ./integration-tests/...

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
	$(CMD_PREFIX) telepresence intercept -n nexodus $(word 2,$(subst _, ,$(basename $@))) --port $(word 3,$(subst _, ,$(basename $@))) --env-json=$(word 2,$(subst _, ,$(basename $@)))-envs.json
	@echo "======================================================================================="
	@echo
	@echo "   Start the $(word 2,$(subst _, ,$(basename $@))) locally with a debugger with the env variables"
	@echo "   with the values found in: $(word 2,$(subst _, ,$(basename $@)))-envs.json"
	@echo
	@echo "   Hint: use the IDEA EnvFile plugin https://plugins.jetbrains.com/plugin/7861-envfile"
	@echo

.PHONY: debug-apiserver
debug-apiserver: telepresence_apiserver_8080 ## Use telepresence to debug the apiserver deployment
.PHONY: debug-apiserver-stop
debug-apiserver-stop: telepresence-prereqs ## Stop using telepresence to debug the apiserver deployment
	$(CMD_PREFIX) telepresence leave apiserver-nexodus

.PHONY: debug-frontend
debug-frontend: telepresence_frontend_3000 ## Use telepresence to debug the frontend deployment
.PHONY: debug-frontend-stop
debug-frontend-stop: telepresence-prereqs ## Stop using telepresence to debug the frontend deployment
	$(CMD_PREFIX) telepresence leave frontend-nexodus

.PHONY: debug-backend-web
debug-backend-web: telepresence_backend-web_8080 ## Use telepresence to debug the backend-web deployment
.PHONY: debug-backend-web-stop
debug-backend-web-stop: telepresence-prereqs ## Stop using telepresence to debug the backend-web deployment
	$(CMD_PREFIX) telepresence leave backend-web-nexodus

NEXODUS_LOCAL_IP:=`go run ./hack/localip`
.PHONY: run-test-container
TEST_CONTAINER_DISTRO?=ubuntu
run-test-container: ## Run docker container that you can run nexodus in
	$(CMD_PREFIX) docker build -f Containerfile.test -t quay.io/nexodus/test:$(TEST_CONTAINER_DISTRO) --target $(TEST_CONTAINER_DISTRO) .
	$(CMD_PREFIX) docker run --rm -it --network bridge \
		--cap-add SYS_MODULE \
		--cap-add NET_ADMIN \
		--cap-add NET_RAW \
		--add-host try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host api.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--add-host auth.try.nexodus.127.0.0.1.nip.io:$(NEXODUS_LOCAL_IP) \
		--mount type=bind,source=$(shell pwd)/.certs,target=/.certs,readonly \
		quay.io/nexodus/test:$(TEST_CONTAINER_DISTRO) /update-ca.sh

.PHONY: run-sql-apiserver
run-sql-apiserver: ## runs a command line SQL client to interact with the apiserver database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl exec -it -n nexodus \
		$(shell kubectl get pods -l postgres-operator.crunchydata.com/role=master -o name) \
		-c database -- psql apiserver
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/postgres -c postgres -- psql -U apiserver apiserver
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/cockroachdb -- cockroach sql --insecure --user apiserver --database apiserver
endif

.PHONY: run-sql-ipam
run-sql-ipam: ## runs a command line SQL client to interact with the ipammake  database
ifeq ($(OVERLAY),dev)
	$(CMD_PREFIX) kubectl exec -it -n nexodus \
		$(shell kubectl get pods -l postgres-operator.crunchydata.com/role=master -o name) \
		-c database -- psql ipam
else ifeq ($(OVERLAY),arm64)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/postgres -c postgres -- psql -U ipam ipam
else ifeq ($(OVERLAY),cockroach)
	$(CMD_PREFIX) kubectl exec -it -n nexodus svc/cockroachdb -- cockroach sql --insecure --user ipam --database ipam
endif

.PHONY: clear-db
clear-db:
	$(CMD_PREFIX) echo "\
		  DROP TABLE IF EXISTS invitations;\
		  DROP TABLE IF EXISTS devices;\
		  DROP TABLE IF EXISTS user_organizations;\
		  DROP TABLE IF EXISTS organizations;\
		  DROP TABLE IF EXISTS users;\
		  DROP TABLE IF EXISTS apiserver_migrations;\
		  " | make run-sql-apiserver 2> /dev/null

##@ Container Images

.PHONY: test-images
test-images: dist/nexd dist/nexctl ## Create test images for e2e
	docker build -f Containerfile.test -t quay.io/nexodus/test:alpine --target alpine .
	docker build -f Containerfile.test -t quay.io/nexodus/test:fedora --target fedora .
	docker build -f Containerfile.test -t quay.io/nexodus/test:ubuntu --target ubuntu .

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
image-nexd:
	docker build -f Containerfile.nexd -t quay.io/nexodus/nexd:$(TAG) .
	docker tag quay.io/nexodus/nexd:$(TAG) quay.io/nexodus/nexd:latest

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
	$(CMD_PREFIX) kubectl rollout restart deploy/apiserver -n nexodus
	$(CMD_PREFIX) kubectl rollout restart deploy/ipam -n nexodus
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

.PHONY: image-mock
image-mock:
	docker build -f Containerfile.mock -t quay.io/nexodus/mock:$(TAG) .
	docker tag quay.io/nexodus/mock:$(TAG) quay.io/nexodus/mock:latest

MOCK_ROOT?=fedora-37-x86_64
SRPM_DISTRO?=fc37

.PHONY: srpm
srpm: dist/rpm image-mock manpages ## Build a source RPM
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
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:latest \
		mock --buildsrpm -D "_commit ${NEXODUS_RELEASE}" --resultdir=/nexodus/dist/rpm/mock --no-clean --no-cleanup-after \
		--spec /nexodus/contrib/rpm/nexodus.spec --sources /nexodus/dist/rpm/ --root ${MOCK_ROOT}
	rm -f dist/rpm/nexodus-${NEXODUS_RELEASE}.tar.gz

.PHONY: rpm
rpm: srpm ## Build an RPM
	docker run --name mock --rm --privileged=true -v $(CURDIR):/nexodus quay.io/nexodus/mock:latest \
		mock --rebuild --without check --resultdir=/nexodus/dist/rpm/mock --root ${MOCK_ROOT} --no-clean --no-cleanup-after \
		/nexodus/$(wildcard dist/rpm/mock/nexodus-0-0.1.$(shell date --utc +%Y%m%d)git$(NEXODUS_RELEASE).$(SRPM_DISTRO).src.rpm)

##@ Documentation

contrib/man:
	$(CMD_PREFIX) mkdir -p contrib/man

.PHONY: manpages
manpages: contrib/man dist/nexd dist/nexctl image-mock ## Generate manpages in ./contrib/man
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
