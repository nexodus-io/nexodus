package messages

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

const (
	ZoneChannelController     = "controller"
	ZoneChannelDefault        = "default"
	HealthcheckRequestChannel = "controller-healthcheck-request"
	HealthcheckReplyChannel   = "controller-healthcheck-reply"
	HealthcheckRequestMsg     = "controller-ready-request"
	HealthcheckReplyMsg       = "controller-healthy"
	RegisterNodeRequest       = "register-node-request"
	RegisterNodeReply         = "register-node-reply"
)

// A pub/sub message
type Message struct {
	Event string
	Peer  Peer
}

type EventType string

const (
	Error EventType = "error"
	Warn  EventType = "warn"
)

type ErrorCode string

const (
	ChannelNotRegistered ErrorCode = "E404"
)

type ErrorMessage struct {
	Event EventType
	Code  ErrorCode
	Msg   string
}

// Peer pub/sub struct
type Peer struct {
	PublicKey   string `json:"public-key"`
	ZoneID      string `json:"zone-id"`
	EndpointIP  string `json:"endpoint-ip"`
	AllowedIPs  string `json:"allowed-ips"`
	NodeAddress string `json:"node-address"`
	ChildPrefix string `json:"child-prefix"`
	HubRouter   bool   `json:"hub-router"`
	HubZone     bool   `json:"hub-zone"`
	ZonePrefix  string `json:"zone-prefix"`
}

func NewPublishPeerMessage(event, zone, pubKey, endpointIP, requestedIP, childPrefix, zonePrefix string, hubZone, hubRouter bool) string {
	msg := Message{}
	msg.Event = event
	peer := Peer{
		PublicKey:   pubKey,
		EndpointIP:  endpointIP,
		ZoneID:      zone,
		NodeAddress: requestedIP,
		ChildPrefix: childPrefix,
		ZonePrefix:  zonePrefix,
		HubZone:     hubZone,
		HubRouter:   hubRouter,
	}
	msg.Peer = peer
	jMsg, _ := json.Marshal(&msg)
	return string(jMsg)
}

type PeerListing []Peer

// handleMsg deal with streaming messages
func HandlePeerList(payload string) ([]Peer, error) {
	var peerListing []Peer
	err := json.Unmarshal([]byte(payload), &peerListing)
	if err != nil {
		log.Debugf("Failed to unmarshal peer listening : %s,  error: %v\n", payload, err)
		return nil, err
	}
	return peerListing, nil
}

func HandleMessage(payload string) Message {
	var msg Message
	err := json.Unmarshal([]byte(payload), &msg)
	if err != nil {
		log.Debugf("Failed to unmarshal message : %s,  error: %v\n", payload, err)
		return msg
	}
	return msg
}

func HandleErrorMessage(payload string) (ErrorMessage, error) {
	var errMsg ErrorMessage
	err := json.Unmarshal([]byte(payload), &errMsg)
	if err != nil {
		log.Debugf("Failed to unmarshal error-message payload %s,  %v\n", payload, err)
		return ErrorMessage{}, err
	}
	return errMsg, nil
}
