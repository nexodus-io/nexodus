package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

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

// cmdListPeers get peer listings from nexd
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

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "PUBLIC KEY", "ENDPOINT", "ALLOWED IPS", "LATEST HANDSHAKE", "TRANSMITTED", "RECEIVED", "HEALTHY")
		}

		for _, peer := range peers {
			tx := strconv.FormatInt(peer.Tx, 10)
			rx := strconv.FormatInt(peer.Rx, 10)
			handshakeTime, err := util.ParseTime(peer.LatestHandshake)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Unable to parse LatestHandshake to time:", err)
			}
			handshake := "None"
			if !handshakeTime.IsZero() {
				secondsAgo := time.Now().UTC().Sub(handshakeTime).Seconds()
				handshake = fmt.Sprintf("%.0f seconds ago", secondsAgo)
			}
			fmt.Fprintf(w, fs, peer.PublicKey, peer.Endpoint, peer.AllowedIPs, handshake, tx, rx, strconv.FormatBool(peer.Healthy))
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, peers)
	if err != nil {
		Fatalf("Failed to print output: %v", err)
	}

	return nil
}
