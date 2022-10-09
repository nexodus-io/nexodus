package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// streamer redis struct
type streamer struct {
	client *redis.Client
}

// dispose streamer instance
func (s *streamer) dispose() {
	s.client.Close()
}

// newPubsub create streamer instance
func newPubsub(client *redis.Client) *streamer {
	return &streamer{client: client}
}

// publishPeers a message to the channel
func (s *streamer) publishPeers(ctx context.Context, channel string, data []Peer) (int64, error) {
	_, msg := createAllPeerMessage(data)
	log.Printf("Published new message: %s\n", msg)
	return s.client.Publish(ctx, channel, msg).Result()
}

// subscribe a redis channel to receive message
func (s *streamer) subscribe(ctx context.Context, channel string, msg chan string) {
	sub := s.client.Subscribe(ctx, channel)
	go func() {
		for {
			outPut, _ := sub.ReceiveMessage(ctx)
			msg <- outPut.Payload
		}
	}()
}

// newRedisClient creates a new redis client instance
func newRedisClient(streamerSocket, streamPasswd string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     streamerSocket,
		Password: streamPasswd,
		DB:       0,
	})
}

// readyCheckRepsonder listens for any msg on healthcheckRequestChannel
// replies on healthcheckReplyChannel to let the agents know it is available
func readyCheckRepsonder(ctx context.Context, client *redis.Client) {
	subHealthRequests := newPubsub(client)
	msgRedChan := make(chan string)
	go func() {
		subHealthRequests.subscribe(ctx, healthcheckRequestChannel, msgRedChan)
		for {
			serverStatusRequest := <-msgRedChan
			fmt.Println(serverStatusRequest)
			if serverStatusRequest == "supervisor-ready-request" {
				client.Publish(ctx, healthcheckReplyChannel, healthcheckReplyMsg).Result()
			}
		}
	}()
}

func createAllPeerMessage(postData []Peer) (string, string) {
	id := uuid.NewString()
	msg, _ := json.Marshal(postData)
	return id, string(msg)
}

func unmarshalMessage(s string) (*MsgEvent, error) {
	var msg MsgEvent
	if err := json.Unmarshal([]byte(s), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
