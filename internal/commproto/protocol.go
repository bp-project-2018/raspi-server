package commproto

// This file manages the flow of datagrams.

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// PubSubClient provides an interface to a publish/subscribe system which is
// used by this package to send and receive the encrypted datagrams.
type PubSubClient interface {
	// Disconnect closes the connection with the pub/sub server.
	Disconnect()
	// Subscribe registers the callback function with the given channel. When a
	// messages is published on the channel, the callback function will be
	// called from a new goroutine. Multiple subscriptions on one channel are
	// possible. Callback should not be nil.
	Subscribe(channel string, callback PubSubCallback)
	// Unsubscribe unregisters all callbacks that were registered to the given
	// channel.
	Unsubscribe(channel string)
	// Publish sends a message to a channel. Message delivery is not guaranteed.
	Publish(channel string, data []byte)
}

// PubSubCallback is a callback function to handle an incoming message on a
// channel.
type PubSubCallback func(channel string, data []byte)

type TimeServer struct {
	config TimeConfiguration
	ps     PubSubClient
}

func NewTimeServer(config TimeConfiguration, ps PubSubClient) *TimeServer {
	return &TimeServer{
		config: config,
		ps:     ps,
	}
}

func (server *TimeServer) Run() {
	log.WithFields(log.Fields{"addr": server.config.Address}).Debug("Starting time server")
	server.ps.Subscribe(fmt.Sprintf("%s/time/request", server.config.Address), server.onRequest)
}

func (server *TimeServer) onRequest(channel string, _ []byte) {
	// @Todo: @Security: Maybe we should start dropping requests, if they come
	// in too fast, to prevent a DOS attack.
	timestamp := int32(time.Now().Unix())
	data := AssembleTime(timestamp, server.config.Passphrase)
	server.ps.Publish(fmt.Sprintf("%s/time", server.config.Address), data)
	log.WithFields(log.Fields{"addr": server.config.Address, "timestamp": timestamp}).Debug("Time server sent time")
}
