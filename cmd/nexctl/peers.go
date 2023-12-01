package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/maps"
	"os"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/urfave/cli/v2"
)

type WgSession struct {
	PublicKey       string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake string
	Tx              int64
	Rx              int64
	Healthy         bool
}

type ListPeersResponse struct {
	RelayPresent  bool                 `json:"relay-present"`
	RelayRequired bool                 `json:"relay-required"`
	Peers         map[string]WgSession `json:"peers"`
}

func peerTableFields(ctx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
	fields = append(fields, TableField{Header: "ENDPOINT", Field: "Endpoint"})
	fields = append(fields, TableField{Header: "ALLOWED IPS", Field: "AllowedIPs"})
	fields = append(fields, TableField{Header: "LATEST HANDSHAKE", Formatter: func(item interface{}) string {
		peer := item.(WgSession)
		handshakeTime, err := util.ParseTime(peer.LatestHandshake)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to parse LatestHandshake to time:", err)
		}
		handshake := "None"
		if !handshakeTime.IsZero() {
			secondsAgo := time.Now().UTC().Sub(handshakeTime).Seconds()
			handshake = fmt.Sprintf("%.0f seconds ago", secondsAgo)
		}
		return handshake
	}})
	fields = append(fields, TableField{Header: "TRANSMITTED", Field: "Tx"})
	fields = append(fields, TableField{Header: "RECEIVED", Field: "Rx"})
	fields = append(fields, TableField{Header: "HEALTHY", Field: "Healthy"})
	return fields
}

// cmdListPeers get peer listings from nexd
func cmdListPeers(cCtx *cli.Context) error {
	var err error
	var response ListPeersResponse
	if err = checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("ListPeers", "")
	if err != nil {
		return fmt.Errorf("Failed to list peers: %w\n", err)
	}

	err = json.Unmarshal([]byte(result), &response)
	if err != nil {
		return fmt.Errorf("Failed to marshall peer results: %w\n", err)
	}

	show(cCtx, peerTableFields(cCtx), maps.Values(response.Peers))
	if response.RelayRequired && !response.RelayPresent {
		fmt.Fprintf(os.Stderr, "\nWARNING: A relay note is required but not present. Connectivity will be limited to devices on the same local network. See https://docs.nexodus.io/user-guide/relay-nodes/\n")
	}

	return nil
}
