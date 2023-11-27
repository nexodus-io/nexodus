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

// cmdListPeers get peer listings from nexd
func cmdListPeers(cCtx *cli.Context, encodeOut string) error {
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

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "PUBLIC KEY", "ENDPOINT", "ALLOWED IPS", "LATEST HANDSHAKE", "TRANSMITTED", "RECEIVED", "HEALTHY")
		}

		for _, peer := range response.Peers {
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

		if encodeOut != encodeNoHeader {
			// If we're not printing the header, something is probably trying to parse the output
			if response.RelayRequired && !response.RelayPresent {
				fmt.Fprintf(w, "\nWARNING: A relay note is required but not present. Connectivity will be limited to devices on the same local network.\n")
			}
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, response.Peers)
	if err != nil {
		Fatalf("Failed to print output: %v", err)
	}

	return nil
}
