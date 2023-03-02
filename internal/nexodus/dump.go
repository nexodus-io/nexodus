package nexodus

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// WgSessions wireguard peer session information
type WgSessions struct {
	PublicKey       string
	PreSharedKey    string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake string
	Tx              int
	Rx              int
}

// ShowDump wireguard interface dump
func ShowDump(iface string) (string, error) {
	dumpOut, err := RunCommand("wg", "show", iface, "dump")
	if err != nil {
		return "", fmt.Errorf("failed to dump wireguard peers: %w", err)
	}

	return dumpOut, nil
}

// DumpPeers dump wireguard peers
func DumpPeers(iface string) ([]WgSessions, error) {
	result, err := ShowDump(iface)
	if err != nil {
		return nil, fmt.Errorf("error running wg show %s dump: %w", iface, err)
	}
	r := bufio.NewReader(strings.NewReader(result))
	peers := make([]WgSessions, 0)
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to read wg dump: %w", err)
		}
		column := strings.Split(string(line), "	")
		if len(column) != 8 {
			continue
		}
		publicKey := column[0]
		psk := column[1]
		endpoints := column[2]
		allowedIPs := strings.Split(column[3], ",")
		latestHandshake, err := strconv.Atoi(column[4])
		if err != nil {
			return nil, fmt.Errorf("latest handshake parse failed: %w", err)
		}
		latestHandshakeTime := time.Duration(0)
		if latestHandshake != 0 {
			latestHandshakeTime = time.Since(time.Unix(int64(latestHandshake), 0))
		}
		tx, err := strconv.Atoi(column[5])
		if err != nil {
			return nil, fmt.Errorf("transfer received parse failed: %w", err)
		}
		tr, err := strconv.Atoi(column[6])
		if err != nil {
			return nil, fmt.Errorf("transfer sent parse failed: %w", err)
		}
		peers = append(peers, WgSessions{
			PublicKey:       publicKey,
			PreSharedKey:    psk,
			Endpoint:        endpoints,
			AllowedIPs:      allowedIPs,
			LatestHandshake: latestHandshakeTime.String(),
			Tx:              tx,
			Rx:              tr,
		})
	}

	return peers, nil
}
