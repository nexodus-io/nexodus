#!/bin/sh

set -e

teardown() {
    ./hack/kind/kind.sh down
}
trap teardown EXIT

./hack/kind/kind.sh up
go test -v --tags=integration ./integration-tests/...
