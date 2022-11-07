.PHONY: all
all: go-lint apex controller

.PHONY: apex
apex: dist/apex dist/apex-arm-linux dist/apex-amd64-linux dist/apex-amd64-darwin dist/apex-arm64-darwin dist/apex-amd64-windows

COMMON_DEPS=$(wildcard ./internal/**/*.go) go.sum go.mod

APEX_DEPS=$(COMMON_DEPS) $(wildcard cmd/apex/*.go)

CONTROLLER_DEPS=$(COMMON_DEPS) $(wildcard cmd/apexcontroller/*.go)

dist:
	mkdir -p $@

dist/apex: $(APEX_DEPS) | dist
	CGO_ENABLED=0 go build -o $@ ./cmd/apex

dist/apex-arm-linux: $(APEX_DEPS) | dist
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o $@ ./cmd/apex

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

.PHONY: gen-docs
gen-docs:
	swag init -g ./cmd/apexcontroller/main.go -o ./internal/docs

.PHONY: test-images
test-images:
	docker build -f Containerfile.test -t quay.io/apex/test:alpine --target alpine .
	docker build -f Containerfile.test -t quay.io/apex/test:fedora --target fedora .
	docker build -f Containerfile.test -t quay.io/apex/test:ubuntu --target ubuntu .

OS_IMAGE?="quay.io/apex/test:fedora"

# Runs the CI e2e tests used in github actions
.PHONY: e2e
e2e: dist/apex
	docker compose build
	./tests/e2e-scripts/init-containers.sh -o $(OS_IMAGE)

.PHONY: e2e
go-e2e: dist/apex test-images
	docker compose up --build -d
	go test -v --tags=integration ./integration-tests/...
