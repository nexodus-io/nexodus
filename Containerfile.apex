FROM docker.io/library/golang:1.18-alpine as build

WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build \
    -ldflags="-extldflags=-static" \
    -o apex ./cmd/apex

FROM docker.io/library/alpine:3.16

RUN apk add --no-cache wireguard-tools iputils
COPY --from=build /src/apex /bin/apex
EXPOSE 8080
ENTRYPOINT [ "/apex" ]
