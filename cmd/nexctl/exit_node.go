package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/urfave/cli/v2"
)

type exitNodeOrigin struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
}

func enableExitNodeClient(cCtx *cli.Context) error {
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

func disableExitNodeClient(cCtx *cli.Context) error {
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

func listExitNodes(cCtx *cli.Context, encodeOut string) error {
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

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "ENDPOINT ADDRESS", "PUBLIC KEY")
		}

		for _, node := range exitNodes {
			fmt.Fprintf(w, fs, node.Endpoint, node.PublicKey)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, exitNodes)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
