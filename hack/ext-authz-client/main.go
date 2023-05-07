package main

import (
	"context"
	"fmt"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/nexodus-io/nexodus/internal/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
)

func check(server string) error {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return err
	}
	defer util.IgnoreError(conn.Close)
	c := auth.NewAuthorizationClient(conn)

	response, err := c.Check(context.Background(), &auth.CheckRequest{})
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", response)
	return nil
}

func main() {
	server := "localhost:5080"
	if len(os.Args) > 1 {
		server = os.Args[1]
	}
	err := check(server)
	if err != nil {
		panic(err)
	}
}
