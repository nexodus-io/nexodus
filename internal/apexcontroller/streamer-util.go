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
