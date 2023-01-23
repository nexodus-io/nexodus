package main

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"

	"github.com/urfave/cli/v2"
)

func callApex(method string) (string, error) {
	conn, err := net.Dial("unix", "/run/apex.sock")
	if err != nil {
		fmt.Printf("Failed to connect to apexd: %+v", err)
		return "", err
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("ApexCtl."+method, nil, &result)
	if err != nil {
		fmt.Printf("Failed to execute method (%s): %+v", method, err)
		return "", err
	}
	return result, nil
}

func checkVersion() error {
	result, err := callApex("Version")
	if err != nil {
		fmt.Printf("Failed to get apexd version: %+v\n", err)
		return err
	}

	if Version != result {
		errMsg := fmt.Sprintf("Version mismatch: apexctl(%s) apexd(%s)\n", Version, result)
		fmt.Print(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func cmdLocalVersion(cCtx *cli.Context) error {
	fmt.Printf("apex version: %s\n", Version)

	result, err := callApex("Version")
	if err == nil {
		fmt.Printf("apexd version: %s\n", result)
	}
	return err
}

func cmdLocalStatus(cCtx *cli.Context) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callApex("Status")
	if err == nil {
		fmt.Print(result)
	}
	return err
}
