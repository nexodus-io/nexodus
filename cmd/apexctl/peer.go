package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
	"github.com/redhat-et/apex/internal/models"
)

func listPeersInZone(c *client.Client, encodeOut, zoneID string) error {
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", zoneID, err)
	}

	peers, err := c.GetZonePeers(zoneUUID)
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "PEER ID", "NODE ADDRESS", "ENDPOINT IP", "REFLEXIVE IPv4", "ALLOWED IPS", "RELAY NODE", "CHILD PREFIX")
		}
		for _, peer := range peers {
			fmt.Fprintf(w, fs, peer.ID, peer.NodeAddress, peer.EndpointIP, peer.ReflexiveIPv4, peer.AllowedIPs, strconv.FormatBool(peer.HubRouter), peer.ChildPrefix)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, peers)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func listAllPeers(c *client.Client, encodeOut string) error {
	allPeers := getAllPeers(c)

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "PEER ID", "NODE ADDRESS", "ENDPOINT IP", "REFLEXIVE IPv4", "ALLOWED IPS", "RELAY NODE", "ZONE ID", "CHILD PREFIX")
		}

		for _, peer := range allPeers {
			fmt.Fprintf(w, fs, peer.ID, peer.NodeAddress, peer.EndpointIP, peer.ReflexiveIPv4, peer.AllowedIPs, strconv.FormatBool(peer.HubRouter), peer.ZoneID, peer.ChildPrefix)
		}

		w.Flush()

		return nil
	}

	err := FormatOutput(encodeOut, allPeers)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func getAllPeers(c *client.Client) []models.Peer {
	zones, err := c.ListZones()
	if err != nil {
		log.Fatal(err)
	}

	var allPeers []models.Peer
	for _, zone := range zones {
		peers, err := c.GetZonePeers(zone.ID)
		if err != nil {
			log.Fatal(err)
		}
		allPeers = append(allPeers, peers...)
	}

	return allPeers
}

func deletePeer(c *client.Client, encodeOut, peerID string) error {
	peerUUID, err := uuid.Parse(peerID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", peerUUID, err)
	}

	res, err := c.DeletePeer(peerUUID)
	if err != nil {
		log.Fatalf("peer delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted peer %s\n", res.ID.String())
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
