package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/urfave/cli/v2"
)

const (
	v6 = "v6"
	v4 = "v4"
)

// cmdConnStatus check the reachability of the node's peers and sort the return by hostname
func cmdConnStatus(cCtx *cli.Context, family string) error {
	if err := checkVersion(); err != nil {
		return err
	}

	// start spinner
	s := spinner.New(spinner.CharSets[70], 100*time.Millisecond)
	s.Suffix = " Running Probe..."
	s.Start()

	result, err := callNexdKeepalives(family)
	if err != nil {
		// clear spinner on error return
		fmt.Print("\r \r")
		return err
	}
	// stop spinner
	s.Stop()
	// clear spinner
	fmt.Print("\r \r")

	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	checkmark := green("✓")
	crossmark := red("x")

	if err == nil {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\n"
		fmt.Fprintf(w, fs, "HOSTNAME", "WIREGUARD ADDRESS", "LATENCY", "CONNECTION STATUS", "PEERING METHOD")

		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			return result[keys[i]].Hostname < result[keys[j]].Hostname
		})

		for _, k := range keys {
			v := result[k]
			status := fmt.Sprintf("%s Unreachable", crossmark)
			if v.IsReachable {
				status = fmt.Sprintf("%s Reachable", checkmark)
			}
			fmt.Fprintf(w, fs, v.Hostname, k, v.Latency, status, v.Method) // Added v.Latency to the output
		}

		w.Flush()
	}

	return err
}

// callNexdKeepalives call the Connectivity ctl methods in nexd agent
func callNexdKeepalives(family string) (map[string]nexodus.KeepaliveStatus, error) {
	var result map[string]nexodus.KeepaliveStatus
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
