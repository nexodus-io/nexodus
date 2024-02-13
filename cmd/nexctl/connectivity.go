package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/nexodus-io/nexodus/internal/api"
	"golang.org/x/exp/maps"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/urfave/cli/v3"
)

const (
	v6 = "v6"
	v4 = "v4"
)

func keepaliveStatusTableFields(command *cli.Command) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	fields = append(fields, TableField{Header: "WIREGUARD ADDRESS", Field: "WgIP"})
	fields = append(fields, TableField{Header: "LATENCY", Field: "Latency"})
	fields = append(fields, TableField{Header: "PEERING METHOD", Field: "Method"})
	fields = append(fields, TableField{Header: "CONNECTION STATUS", Formatter: func(item interface{}) string {
		green := color.New(color.FgGreen).SprintFunc()
		red := color.New(color.FgRed).SprintFunc()
		v := item.(api.KeepaliveStatus)
		status := fmt.Sprintf("%s Unreachable", red("x"))
		if v.IsReachable {
			status = fmt.Sprintf("%s Reachable", green("✓"))
		}
		return status
	}})
	return fields
}

// cmdConnStatus check the reachability of the node's peers and sort the return by hostname
func cmdConnStatus(ctx context.Context, command *cli.Command, family string) error {
	if err := checkVersion(); err != nil {
		return err
	}

	var s *spinner.Spinner
	if command.String("output") == encodeColumn {
		// start spinner, but only for human readable output format,
		// not when generating parseable output.
		s = spinner.New(spinner.CharSets[70], 100*time.Millisecond)
		s.Suffix = " Running Probe..."
		s.Start()
	}

	result, err := callNexdKeepalives(family)
	if err != nil {
		// clear spinner on error return
		fmt.Print("\r \r")
		return err
	}
	if s != nil {
		// stop spinner
		s.Stop()
	}
	// clear spinner
	fmt.Print("\r \r")

	peers := maps.Values(result.Peers)
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Hostname < peers[j].Hostname
	})
	show(command, keepaliveStatusTableFields(command), peers)

	if result.RelayRequired && !result.RelayPresent {
		fmt.Fprintf(os.Stderr, "\nWARNING: A relay node is required but not present. Connectivity will be limited to devices on the same local network. See https://docs.nexodus.io/user-guide/relay-nodes/\n")
	}
	return nil
}

// callNexdKeepalives call the Connectivity ctl methods in nexd agent
func callNexdKeepalives(family string) (api.PingPeersResponse, error) {
	var result api.PingPeersResponse
	var keepaliveJson string
	var err error

	if family == v6 {
		keepaliveJson, err = callNexd("ConnectivityV6", "")
		if err != nil {
			return result, fmt.Errorf("Failed to get nexd connectivity status: %w\n", err)
		}
	} else {
		keepaliveJson, err = callNexd("ConnectivityV4", "")
		if err != nil {
			return result, fmt.Errorf("Failed to get nexd connectivity status: %w\n", err)
		}
	}

	err = json.Unmarshal([]byte(keepaliveJson), &result)
	if err != nil {
		return result, fmt.Errorf("Failed to get unmarshall connecitivty results: %w\n", err)
	}

	return result, nil
}
