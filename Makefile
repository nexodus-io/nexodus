.PHONY: all
all: go-lint apex controller

.PHONY: apex
apex: dist/apex dist/apex-amd64-linux dist/apex-amd64-darwin dist/apex-arm64-darwin dist/apex-amd64-windows

COMMON_DEPS=$(wildcard ./internal/messages/*.go) $(wildcard ./internal/streamer/*.go) go.sum go.mod

APEX_DEPS=$(COMMON_DEPS) $(wildcard cmd/apex/*.go) $(wildcard internal/apex/*.go)

CONTROLLER_DEPS=$(COMMON_DEPS) $(wildcard cmd/apexcontroller/*.go) $(wildcard internal/apexcontroller/*.go)

dist:
	mkdir -p $@

dist/apex: $(APEX_DEPS) | dist
	CGO_ENABLED=0 go build -o $@ ./cmd/apex

dist/apex-amd64-linux: $(APEX_DEPS) | dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/apex

dist/apex-amd64-darwin: $(APEX_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ ./cmd/apex

dist/apex-arm64-darwin: $(APEX_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ ./cmd/apex

dist/apex-amd64-windows: $(APEX_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ ./cmd/apex

.PHONY: clean
clean:
	rm -rf dist

.PHONY: controller
controller: dist/apexcontroller dist/apexcontroller-amd64-linux dist/apexcontroller-amd64-darwin dist/apexcontroller-arm64-darwin

dist/apexcontroller: $(CONTROLLER_DEPS) | dist
	CGO_ENABLED=0 go build -o $@ ./cmd/apexcontroller

dist/apexcontroller-amd64-linux: $(CONTROLLER_DEPS) | dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/apexcontroller

dist/apexcontroller-amd64-darwin: $(CONTROLLER_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $@ ./cmd/apexcontroller

dist/apexcontroller-arm64-darwin: $(CONTROLLER_DEPS) | dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $@ ./cmd/apexcontroller

.PHONY: go-lint
go-lint: $(APEX_DEPS) $(CONTROLLER_DEPS)
	@if ! which golangci-lint 2>&1 >/dev/null; then \
		echo "Please install golangci-lint." ; \
		echo "See: https://golangci-lint.run/usage/install/#local-installation" ; \
		exit 1 ; \
	fi
	golangci-lint run ./...
# CI infrastructure setup and tests triggered by actions workflow

.PHONY: test-images
test-images:
	docker build -f tests/Containerfile.alpine -t quay.io/apex/test:alpine tests
	docker build -f tests/Containerfile.fedora -t quay.io/apex/test:fedora tests
	docker build -f tests/Containerfile.ubuntu -t quay.io/apex/test:ubuntu tests

OS_IMAGE?="quay.io/apex/test:fedora"

# Runs the CI e2e tests used in github actions
.PHONY: run-ci-e2e
run-ci-e2e: dist/apex
	docker compose build
	./tests/e2e-scripts/init-containers.sh -o $(OS_IMAGE)
