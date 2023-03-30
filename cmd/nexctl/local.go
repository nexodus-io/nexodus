package main

import (
	"fmt"
	"net"
	"net/rpc/jsonrpc"
	"runtime"

	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/urfave/cli/v2"
)

func callNexd(method string) (string, error) {
	conn, err := net.Dial("unix", "/run/nexd.sock")
	if err != nil {
		return "", fmt.Errorf("Failed to connect to nexd: %w\n", err)
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)

	var result string
	err = client.Call("NexdCtl."+method, nil, &result)
	if err != nil {
		return "", fmt.Errorf("Failed to execute method (%s): %w\n", method, err)
	}

	return result, nil
}

func checkVersion() error {
	result, err := callNexd("Version")
	if err != nil {
		return fmt.Errorf("Failed to get nexd version: %w\n", err)
	}

	if Version != result {
		errMsg := fmt.Sprintf("Version mismatch: nexctl(%s) nexd(%s)\n", Version, result)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func cmdLocalVersion(cCtx *cli.Context) error {
	fmt.Printf("nexctl version: %s\n", Version)
	if runtime.GOOS != nexodus.Linux.String() {
		return fmt.Errorf("nexd ctl interface not currently supported on %s", runtime.GOOS)
	}

	result, err := callNexd("Version")
	if err == nil {
		fmt.Printf("nexd version: %s\n", result)
	}

	return err
}

func cmdLocalStatus(cCtx *cli.Context) (string, error) {
	if runtime.GOOS != nexodus.Linux.String() {
		return "", fmt.Errorf("nexd ctl interface not yet supported on %s", runtime.GOOS)
	}

	if err := checkVersion(); err != nil {
		return "", err
	}

	result, err := callNexd("Status")
	if err != nil {
		return "", err
	}

	return result, err
}
