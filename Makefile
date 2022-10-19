#Aircrew Targets
build-aircrew-local:
	go build -o ./cmd/aircrew/bin/aircrew ./cmd/aircrew
build-aircrew-linux:
	GOOS=linux GOARCH=amd64 go build -o ./cmd/aircrew/bin/aircrew-amd64-linux ./cmd/aircrew
build-aircrew-darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./cmd/aircrew/bin/aircrew-amd64-darwin ./cmd/aircrew
clean-aircrew:
	rm -Rf ./cmd/aircrew/bin

#ControlTower Targets
build-controltower-local:
	go build -o ./cmd/controltower/bin/controltower ./cmd/controltower
build-controltower-linux:
	GOOS=linux GOARCH=amd64 go build -o ./cmd/controltower/bin/controltower-amd64-linux ./cmd/controltower
build-controltower-darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./cmd/controltower/bin/controltower-amd64-darwin ./cmd/controltower
clean-controltower:
	rm -Rf ./cmd/controltower/bin

#Lint

go-lint:
	golangci-lint run ./...
# CI infrastructure setup and tests triggered by actions workflow

# Runs the CI e2e tests used in github actions
run-ci-e2e:
	./tests/e2e-scripts/init-containers.sh -o $(OS_IMAGE)
