package streamer

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

const (
	pubSubPort = 6379
)

type Streamer struct {
	sCtx          context.Context
	client        *redis.Client
	addr          string
	port          int
	subscriptions map[string]*redis.PubSub
}

type ReceivedMessage struct {
	Channel string
	Payload string
}

// New Streamer instance.
func NewStreamer(ctx context.Context, ip string, port int, password string) *Streamer {
	streamerUrl := fmt.Sprintf("%s:%d", ip, port)
	rc := redis.NewClient(&redis.Options{
		Addr:     streamerUrl,
		Password: password,
	})
	stm := &Streamer{
		sCtx:          ctx,
		client:        rc,
		addr:          ip,
		port:          pubSubPort,
		subscriptions: make(map[string]*redis.PubSub),
	}

	return stm
}

// Check is streamer instance is ready to receive messages
func (stm *Streamer) IsReady() bool {
	_, err := stm.client.Ping(stm.sCtx).Result()
	if err != nil {
		log.Errorf("Unable to connect to the streamer instance at %s: %v", stm.addr, err)
		return false
	}
	return true
}

func (stm *Streamer) SubscribeAndReceive(channel string, subChan chan ReceivedMessage) {
	log.Debugf("Received request to subscribe to channel [%s]", channel)
	sub := stm.client.Subscribe(stm.sCtx, channel)
	stm.subscriptions[channel] = sub
	go func() {
		for {
			output, err := sub.ReceiveMessage(stm.sCtx)
			if err != nil {
				log.Errorf("error in receiving message : %v", err)
			} else {
				msg := ReceivedMessage{
					Channel: output.Channel,
					Payload: output.Payload,
				}
				subChan <- msg
			}
		}
	}()
}

func (stm *Streamer) PublishMessage(channel string, message string) error {
	log.Debugf("Publishing on channel [%s] : %s", channel, message)
	_, err := stm.client.Publish(stm.sCtx, channel, message).Result()
	return err
}

func (stm *Streamer) Close() {
	log.Debugf("Closing stream instance at %s:%d", stm.addr, stm.port)
	for _, sub := range stm.subscriptions {
		sub.Close()
	}
	stm.client.Close()
}

func (stm *Streamer) CloseSubscription(channel string) {
	if sub, ok := stm.subscriptions[channel]; ok {
		sub.Close()
	}
}

func (stm *Streamer) GetUrl() string {
	return stm.addr
}
