FROM docker.io/library/golang:1.19-alpine as build

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-extldflags=-static" \
    -o go-oidc-agent ./cmd/go-oidc-agent

FROM registry.access.redhat.com/ubi8/ubi

COPY --from=build /src/go-oidc-agent /go-oidc-agent
EXPOSE 8080
ENTRYPOINT [ "/go-oidc-agent" ]
