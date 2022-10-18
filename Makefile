# CI infrastructure setup and tests triggered by actions workflow

# Runs the CI e2e tests used in github actions
run-ci-e2e:
	./tests/e2e-scripts/init-containers.sh -o $(OS_IMAGE)
