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
	Disconnect()
	Subscribe(channel string, callback PubSubCallback)
	Unsubscribe(channel string)
	Publish(channel string, data []byte)
}

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
