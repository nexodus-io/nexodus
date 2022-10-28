package apexcontroller

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/messages"
	"github.com/redhat-et/apex/internal/streamer"
	log "github.com/sirupsen/logrus"
)

// publishPeers a message to the channel
func publishPeers(stm *streamer.Streamer, channel string, data []messages.Peer) error {
	_, msg := createAllPeerMessage(data)
	log.Printf("Published new message to channel %s: %s\n", channel, msg)
	return stm.PublishMessage(channel, msg)
}

// readyCheckResponder listens for any msg on healthcheckRequestChannel
// replies on healthcheckReplyChannel to let the agents know it is available
func readyCheckResponder(ctx context.Context, stm *streamer.Streamer, readyChan chan streamer.ReceivedMessage, wg *sync.WaitGroup) {
	stm.SubscribeAndReceive(messages.HealthcheckRequestChannel, readyChan)
	wg.Add(1)
	go func() {
		for {
			serverStatusRequest, ok := <-readyChan
			if !ok {
				wg.Done()
				return
			}
			log.Debugf("Ready check channel message : %v", serverStatusRequest)
			if serverStatusRequest.Payload == "controller-ready-request" {
				if err := stm.PublishMessage(messages.HealthcheckReplyChannel, messages.HealthcheckReplyMsg); err != nil {
					log.Errorf("unable to publish healthcheck reply: %v", err)
				}
			}
		}
	}()
}

func createAllPeerMessage(postData []messages.Peer) (string, string) {
	id := uuid.NewString()
	msg, _ := json.Marshal(postData)
	return id, string(msg)
}

func publishErrorMessage(stm *streamer.Streamer, channel string, event messages.EventType, code messages.ErrorCode, msg string) error {
	jsonMsg := createErrorMessage(event, code, msg)
	log.Printf("Published new error message to channel %s : %s\n", channel, jsonMsg)
	return stm.PublishMessage(channel, jsonMsg)
}

func createErrorMessage(event messages.EventType, code messages.ErrorCode, msg string) string {
	errMsg := messages.ErrorMessage{}
	errMsg.Event = event
	errMsg.Code = code
	errMsg.Msg = msg
	jMsg, _ := json.Marshal(&errMsg)
	return string(jMsg)
}
