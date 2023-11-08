#!/bin/bash
set -e
if [ -z "$(which kubectl)" ] || [ "$1" = "--force" ]; then
  echo installing kubectl
  if [[ $(uname -s) == "Linux" ]]; then
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
  elif [[ $(uname -s) == "Darwin" ]]; then
    echo "Operating System: macOS"
    arch=$(uname -m)
    case $arch in
    "x86_64")
      echo "Architecture: amd64"
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/amd64/kubectl"
      ;;
    "arm64")
      echo "Architecture: arm64"
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
      ;;
    *)
      echo "Unknown Architecture: $arch"
      exit 1
      ;;
    esac
  else
    echo "Unknown OS"
    exit 1
  fi
  chmod 755 kubectl
  mkdir -p "$(go env GOPATH)/bin"
  mv kubectl "$(go env GOPATH)/bin"
fi
if [ -z "$(which kind)" ] || [ "$1" = "--force" ]; then
  echo installing kind
  go install sigs.k8s.io/kind@v0.17.0
fi
if [ -z "$(which golangci-lint)" ] || [ "$1" = "--force" ]; then
  echo installing golangci-lint
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2
fi
if [ -z "$(which swag)" ] || [ "$1" = "--force" ]; then
  echo installing swag
  go install github.com/swaggo/swag/cmd/swag@v1.16.1
fi
if [ -z "$(which mkcert)" ] || [ "$1" = "--force" ]; then
  echo installing mkcert
  go install filippo.io/mkcert@v1.4.4
fi
if [ -z "$(which kubeconform)" ] || [ "$1" = "--force" ]; then
  echo installing kubeconform
  go install github.com/yannh/kubeconform/cmd/kubeconform@v0.5.0
fi
if [ -z "$(which kustomize)" ] || [ "$1" = "--force" ]; then
  echo installing kustomize
  go install sigs.k8s.io/kustomize/kustomize/v5@latest
fi
if [ -z "$(which go-licenses)" ] || [ "$1" = "--force" ]; then
  echo installing go-licenses
  go install github.com/google/go-licenses@latest
fi
if [ -z "$(which gotestsum)" ] || [ "$1" = "--force" ]; then
  echo installing gotestsum
  go install gotest.tools/gotestsum@v1.10.0
fi


