FROM docker.io/library/golang:1.19 as build-apex
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-extldflags=-static" \
    -o apexd ./cmd/apexd

FROM docker.io/library/golang:1.19 as build-mkcert
ARG TARGETOS
ARG TARGETARCH

WORKDIR /
RUN git clone https://github.com/FiloSottile/mkcert && cd mkcert && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-extldflags=-static -X main.Version=$(git describe --tags)"

FROM fedora:36 as fedora

COPY --from=build-apex /src/apexd /apexd
COPY --from=build-mkcert /mkcert/mkcert /mkcert
RUN dnf update -y && \
    dnf install wireguard-tools iproute -y && \
    dnf clean all -y && \
    rm -rf /var/cache/yum
