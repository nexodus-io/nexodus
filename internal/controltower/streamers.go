package controltower

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
)

// streamer redis struct
type streamer struct {
	client *redis.Client
}

// NewPubsub create streamer instance
func NewPubsub(client *redis.Client) *streamer {
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
			output, _ := sub.ReceiveMessage(ctx)
			msg <- output.Payload
		}
	}()
}

// NewRedisClient creates a new redis client instance
func NewRedisClient(streamerSocket, streamPasswd string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     streamerSocket,
		Password: streamPasswd,
		DB:       0,
	})
}

// readyCheckResponder listens for any msg on healthcheckRequestChannel
// replies on healthcheckReplyChannel to let the agents know it is available
func readyCheckResponder(ctx context.Context, client *redis.Client, readyChan chan string, wg *sync.WaitGroup) {
	subHealthRequests := NewPubsub(client)
	wg.Add(1)
	go func() {
		subHealthRequests.subscribe(ctx, messages.HealthcheckRequestChannel, readyChan)
		for {
			serverStatusRequest, ok := <-readyChan
			if !ok {
				wg.Done()
				return
			}
			log.Debugf("Ready check channel message %s", serverStatusRequest)
			if serverStatusRequest == "controltower-ready-request" {
				if _, err := client.Publish(ctx, messages.HealthcheckReplyChannel, messages.HealthcheckReplyMsg).Result(); err != nil {
					log.Errorf("Unable to publish healthcheck reply: %s", err)
				}
			}
		}
	}()
}

func createAllPeerMessage(postData []Peer) (string, string) {
	id := uuid.NewString()
	msg, _ := json.Marshal(postData)
	return id, string(msg)
}
