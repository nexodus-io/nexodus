package main

import (
	"encoding/json"
	"log"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

// streamer redis struct
type streamer struct {
	client *redis.Client
}

// dispose streamer instance
func (pubsub *streamer) dispose() {
	pubsub.client.Close()
}

// newPubsub create streamer instance
func newPubsub(client *redis.Client) *streamer {
	return &streamer{client: client}
}

// publish a message to the channel
func (pubsub *streamer) publish(channel string, data []Peer) (int64, error) {
	_, msg := createAllPeerMessage(data)
	log.Printf("[INFO] Published new message: %s\n", msg)
	return pubsub.client.Publish(channel, msg).Result()
}

// subscribe a redis channel to receive message
func (pubsub *streamer) subscribe(channel string, msg chan string) {
	sub := pubsub.client.Subscribe(channel)
	go func() {
		for {
			outPut, _ := sub.ReceiveMessage()
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
