package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"encoding/json"

	"github.com/urfave/cli/v2"
)

type WgListPeers struct {
	PublicKey       string
	PreSharedKey    string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake string
	Tx              int64
	Rx              int64
}

func cmdListPeers(cCtx *cli.Context, encodeOut string) error {
	var err error
	var peers []WgListPeers
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
		fs := "%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "PUBLIC KEY", "ENDPOINT", "ALLOWED IPS", "LATEST HANDSHAKE", "TRANSMITTED", "RECEIVED")
		}

		for _, peer := range peers {
			tx := strconv.FormatInt(peer.Tx, 10)
			rx := strconv.FormatInt(peer.Rx, 10)
			handshakeTime, err := ParseTime(peer.LatestHandshake)
			if err != nil {
				log.Printf("Unable to parse LatestHandshake to time: %v", err)
			}
			handshake := "None"
			if !handshakeTime.IsZero() {
				secondsAgo := time.Now().UTC().Sub(handshakeTime).Seconds()
				handshake = fmt.Sprintf("%.0f seconds ago", secondsAgo)
			}
			fmt.Fprintf(w, fs, peer.PublicKey, peer.Endpoint, peer.AllowedIPs, handshake, tx, rx)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, peers)
	if err != nil {
		log.Fatalf("Failed to print output: %v", err)
	}

	return nil
}

// ParseTime attempts to parse a time string in three possible formats to handle UTC and local time differences between Darwin and Linux and the proxy mode
func ParseTime(timeStr string) (time.Time, error) {
	var t time.Time
	var ut int64
	var err error
	if t, err = time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return t.UTC(), nil
	}
	if t, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timeStr); err == nil {
		return t.UTC(), nil
	}
	if ut, err = strconv.ParseInt(timeStr, 10, 64); err == nil {
		if ut != 0 {
			t = time.Unix(ut, 0)
		}
	}
	return t.UTC(), err
}
