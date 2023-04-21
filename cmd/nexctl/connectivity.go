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

// cmdConnStatus check the reachability of the node's peers and sort the return by hostname
func cmdConnStatus(cCtx *cli.Context) error {
	if err := checkVersion(); err != nil {
		return err
	}

	// start spinner
	s := spinner.New(spinner.CharSets[70], 100*time.Millisecond)
	s.Suffix = " Running Probe..."
	s.Start()

	result, err := callNexdKeepalives()
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
		fs := "%s\t%s\t%s\n"
		fmt.Fprintf(w, fs, "HOSTNAME", "WIREGUARD ADDRESS", "CONNECTION STATUS")

		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			return result[keys[i]].Hostname < result[keys[j]].Hostname
		})

		for _, k := range keys {
			v := result[k]
			if v.IsReachable {
				fmt.Fprintf(w, fs, v.Hostname, k, fmt.Sprintf("%s Reachable", checkmark))
			} else {
				fmt.Fprintf(w, fs, v.Hostname, k, fmt.Sprintf("%s Unreachable", crossmark))
			}
		}

		w.Flush()
	}

	return err
}

func callNexdKeepalives() (map[string]nexodus.KeepaliveStatus, error) {
	var result map[string]nexodus.KeepaliveStatus

	keepaliveJson, err := callNexd("Connectivity", "")
	if err != nil {
		return result, fmt.Errorf("Failed to get nexd connectivity status: %w\n", err)
	}

	err = json.Unmarshal([]byte(keepaliveJson), &result)
	if err != nil {
		return result, fmt.Errorf("Failed to get unmarshall connecitivty results: %w\n", err)
	}

	return result, nil
}
