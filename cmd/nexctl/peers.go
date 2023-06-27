package main

import (
	"fmt"
	"log"
	"time"

	"encoding/json"

	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/urfave/cli/v2"
)

type WgListPeers struct {
	PublicKey       string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake string
	Tx              int64
	Rx              int64
	Healthy         bool
}

func peerTableFields(cCtx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
	full := cCtx.Bool("full")
	if full {
		fields = append(fields, TableField{Header: "ENDPOINT", Field: "Endpoint"})
	}
	fields = append(fields, TableField{Header: "ALLOWED IPS", Field: "AllowedIPs"})
	if full {
		fields = append(fields, TableField{
			Header: "LATEST HANDSHAKE",
			Formatter: func(item interface{}) string {
				peer := item.(WgListPeers)
				handshakeTime, err := util.ParseTime(peer.LatestHandshake)
				if err != nil {
					log.Printf("Unable to parse LatestHandshake to time: %v", err)
				}
				handshake := "None"
				if !handshakeTime.IsZero() {
					secondsAgo := time.Now().UTC().Sub(handshakeTime).Seconds()
					handshake = fmt.Sprintf("%.0f seconds ago", secondsAgo)
				}
				return handshake
			},
		})
		fields = append(fields, TableField{Header: "TRANSMITTED", Field: "Tx"})
		fields = append(fields, TableField{Header: "RECEIVED", Field: "Rx"})
	}
	fields = append(fields, TableField{Header: "HEALTHY", Field: ""})
	return fields
}

func cmdListPeers(cCtx *cli.Context, encodeOut string) error {
	var err error
	var peers map[string]WgListPeers
	if err = checkVersion(); err != nil {
		return err
	}

	result, err := callNexd("ListPeers", "")
	if err != nil {
		return fmt.Errorf("Failed to list peers: %w\n", err)
	}

	err = json.Unmarshal([]byte(result), &peers)
	if err != nil {
		return fmt.Errorf("Failed to marshall peer results: %w\n", err)
	}

	showOutput(cCtx, peerTableFields(cCtx), peers)
	return nil
}
