#!/bin/bash
set -e
if [ -z "$(which kind)" ] || [ "$1" = "--force" ]; then
  echo installing kind
  go install sigs.k8s.io/kind@v0.17.0
fi
if [ -z "$(which golangci-lint)" ] || [ "$1" = "--force" ]; then
  echo installing golangci-lint
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.0
fi
if [ -z "$(which swag)" ] || [ "$1" = "--force" ]; then
  echo installing swag
  go install github.com/swaggo/swag/cmd/swag@v1.8.10
fi
if [ -z "$(which mkcert)" ] || [ "$1" = "--force" ]; then
  echo installing mkcert
  go install filippo.io/mkcert@v1.4.4
fi
