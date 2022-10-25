# Common Targets
all: build-apex-local build-controller-local

clean: clean-apex clean-controller

#Apex Targets
build-apex-local:
	go build -o ./dist/apex ./cmd/apex
build-apex-linux:
	GOOS=linux GOARCH=amd64 go build -o ./dist/apex-amd64-linux ./cmd/apex
build-apex-darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./dist/apex-amd64-darwin ./cmd/apex
clean-apex:
	rm -Rf ./dist/apex ./dist/apex-amd64-linux ./dist/apex-amd64-darwin

#ControlTower Targets
build-controller-local:
	go build -o ./dist/apexcontroller ./cmd/apexcontroller
build-controller-linux:
	GOOS=linux GOARCH=amd64 go build -o ./dist/apexcontroller-amd64-linux ./cmd/apexcontroller
build-controller-darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./dist/apexcontroller-amd64-darwin ./cmd/apexcontroller
clean-controller:
	rm -Rf ./dist/apexcontroller ./dist/apexcontroller-amd64-linux ./dist/apexcontroller-amd64-darwin
#Lint

go-lint:
	golangci-lint run ./...
# CI infrastructure setup and tests triggered by actions workflow

# Runs the CI e2e tests used in github actions
run-ci-e2e:
	./tests/e2e-scripts/init-containers.sh -o $(OS_IMAGE)

.PHONY: all clean \
	build-apex-local build-apex-linux build-apex-darwin clean-apex \
	build-controller-local build-controller-linux build-controller-darwin clean-controller \
	go-lint run-ci-e2e
