package main

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"

	"github.com/urfave/cli/v2"
)

func callNexd(method string) (string, error) {
	conn, err := net.Dial("unix", "/run/nexd.sock")
	if err != nil {
		fmt.Printf("Failed to connect to nexd: %+v", err)
		return "", err
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("NexdCtl."+method, nil, &result)
	if err != nil {
		fmt.Printf("Failed to execute method (%s): %+v", method, err)
		return "", err
	}
	return result, nil
}

func checkVersion() error {
	result, err := callNexd("Version")
	if err != nil {
		fmt.Printf("Failed to get nexd version: %+v\n", err)
		return err
	}

	if Version != result {
		errMsg := fmt.Sprintf("Version mismatch: nexctl(%s) nexd(%s)\n", Version, result)
		fmt.Print(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func cmdLocalVersion(cCtx *cli.Context) error {
	fmt.Printf("nexctl version: %s\n", Version)

	result, err := callNexd("Version")
	if err == nil {
		fmt.Printf("nexd version: %s\n", result)
	}
	return err
}

func cmdLocalStatus(cCtx *cli.Context) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("Status")
	if err == nil {
		fmt.Print(result)
	}
	return err
}
