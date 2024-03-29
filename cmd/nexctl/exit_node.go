package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v3"
)

type exitNodeOrigin struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
}

func enableExitNodeClient(ctx context.Context, command *cli.Command) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("EnableExitNodeClient", "")
	if err != nil {
		return fmt.Errorf("Failed to enable exit node client: %w\n", err)
	}

	if result == "null" {
		fmt.Printf("Successfully enabled exit node client on this device\n")
		return nil
	}
	fmt.Printf("Error encountered  while enabing exit node client: %s\n", result)

	return nil
}

func disableExitNodeClient(ctx context.Context, command *cli.Command) error {
	if err := checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("DisableExitNodeClient", "")
	if err != nil {
		return fmt.Errorf("Failed to disable exit node client: %w\n", err)
	}

	if result == "null" {
		fmt.Printf("Successfully disabled exit node client on this device\n")
		return nil
	}
	fmt.Printf("Error encountered while enabing exit node client: %s\n", result)

	return nil
}

func exitNodeTableFields(command *cli.Command) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "ENDPOINT ADDRESS", Field: "Endpoint"})
	fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
	return fields
}
func listExitNodes(ctx context.Context, command *cli.Command, encodeOut string) error {
	var err error
	var exitNodes []exitNodeOrigin
	if err = checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("ListExitNodes", "")
	if err != nil {
		return fmt.Errorf("Failed to list exit nodes: %w\n", err)
	}

	err = json.Unmarshal([]byte(result), &exitNodes)
	if err != nil {
		return fmt.Errorf("Failed to marshall exit node results: %w\n", err)
	}

	show(command, exitNodeTableFields(command), exitNodes)
	return nil
}
