package commproto

// This file manages the flow of datagrams.

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
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

type Client struct {
	config ClientConfiguration
	ps     PubSubClient

	timeServer *timeServer
	timeClient *timeClient

	callbacks []DatagramCallback
}

type DatagramCallback func(sender string, data []byte)

func NewClient(config *ClientConfiguration, ps PubSubClient) *Client {
	client := &Client{
		config: *config,
		ps:     ps,
	}
	if config.TimeServer != nil {
		client.timeServer = &timeServer{
			config: *config.TimeServer,
			ps:     ps,
		}
	}
	if config.TimeClient != nil {
		client.timeClient = &timeClient{
			config: *config.TimeClient,
			ps:     ps,
		}
	}
	return client
}

func (client *Client) RegisterCallback(callback DatagramCallback) {
	if callback == nil {
		panic("nil callback")
	}
	client.callbacks = append(client.callbacks, callback)
}

func (client *Client) Start() {
	if client.timeServer != nil {
		client.timeServer.Start()
	}
	if client.timeClient != nil {
		client.timeClient.Start()
	}
	client.ps.Subscribe(fmt.Sprintf("%s/inbox", client.config.HostAddress), client.onDatagram)
}

func (client *Client) onDatagram(_ string, datagram []byte) {
	sender, ok := ExtractAddress(datagram)
	if !ok {
		log.Warn("Received invalid datagram")
		return
	}

	senderConfig, ok := client.config.Partners[sender]
	if !ok {
		log.WithFields(log.Fields{"sender": sender}).Info("Ignoring datagram from unknown sender")
		return
	}

	timestamp64, data, err := DisassembleDatagram(datagram, sender, senderConfig.Key, senderConfig.Passphrase)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Received invalid datagram")
		return
	}

	timestamp := int32(timestamp64 / 1e6) // @Temporary

	current, err := client.getTime()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Failed to ge time while receiving datagram")
		return
	}

	if delta := timestamp - current; delta < -1 || delta > 1 { // @Hardcoded
		log.WithFields(log.Fields{"delta": delta}).Warn("Received datagram with invalid timestamp")
		return
	}

	// @Todo: @Sync: Protect client.callbacks??
	for _, callback := range client.callbacks {
		callback(sender, data)
	}
}

func (client *Client) SendString(receiver string, data string) error {
	return client.Send(receiver, []byte(data))
}

func (client *Client) Send(receiver string, data []byte) error {
	receiverConfig, ok := client.config.Partners[receiver]
	if !ok {
		return fmt.Errorf("unknown receiver: %s", receiver)
	}

	timestamp, err := client.getTime()
	if err != nil {
		return fmt.Errorf("failed to get time: %v", err)
	}
	iv, err := generateSecureRandomByteArray(16)
	if err != nil {
		return fmt.Errorf("failed to generate iv: %v", err)
	}
	datagram := AssembleDatagram(client.config.HostAddress, iv, int64(timestamp)*1e6, data, receiverConfig.Key, receiverConfig.Passphrase)
	client.ps.Publish(fmt.Sprintf("%s/inbox", receiver), datagram)
	return nil
}

func (client *Client) getTime() (timestamp int32, err error) {
	if client.timeClient != nil {
		return client.timeClient.getTime()
	}
	return int32(time.Now().Unix()), nil
}

type timeServer struct {
	config TimeConfiguration
	ps     PubSubClient
}

func (server *timeServer) Start() {
	log.WithFields(log.Fields{"addr": server.config.Address}).Debug("Starting time server")
	server.ps.Subscribe(fmt.Sprintf("%s/time/request", server.config.Address), server.onRequest)
}

func (server *timeServer) onRequest(string, []byte) {
	// @Todo: @Security: Maybe we should start dropping requests, if they come
	// in too fast, to prevent a DOS attack.
	timestamp := int32(time.Now().Unix())
	data := AssembleTime(timestamp, server.config.Passphrase)
	server.ps.Publish(fmt.Sprintf("%s/time", server.config.Address), data)
	log.WithFields(log.Fields{"addr": server.config.Address, "timestamp": timestamp}).Debug("Time server sent time")
}

type timeClient struct {
	config TimeConfiguration
	ps     PubSubClient

	// The time server reported baseTimestamp at local time baseTime.
	baseMutex     sync.Mutex
	baseTimestamp int32
	baseTime      time.Time
}

func (client *timeClient) Start() {
	log.WithFields(log.Fields{"addr": client.config.Address}).Debug("Starting time client")
	client.ps.Subscribe(fmt.Sprintf("%s/time", client.config.Address), client.onTime)
	client.publishRequest()
	go client.requestLoop()
}

func (client *timeClient) onTime(_ string, data []byte) {
	timestamp, err := DisassembleTime(data, client.config.Passphrase)
	if err != nil {
		log.Info("Time client received invalid time datagram")
		return
	}

	client.baseMutex.Lock()
	client.baseTimestamp = timestamp
	client.baseTime = time.Now()
	client.baseMutex.Unlock()
	log.WithFields(log.Fields{"addr": client.config.Address, "timestamp": timestamp}).Debug("Time client received time")
}

func (client *timeClient) publishRequest() {
	log.WithFields(log.Fields{"addr": client.config.Address}).Debug("Time client will send request")
	client.ps.Publish(fmt.Sprintf("%s/time/request", client.config.Address), []byte{}) // empty request
}

func (client *timeClient) requestLoop() {
	for {
		time.Sleep(time.Second)
		client.baseMutex.Lock()
		baseTime := client.baseTime
		client.baseMutex.Unlock()
		if !baseTime.IsZero() {
			return // time received, all is well :)
		}
		client.publishRequest()
	}
}

func (client *timeClient) getTime() (timestamp int32, err error) {
	client.baseMutex.Lock()
	if client.baseTime.IsZero() {
		err = errors.New("no time server connection")
	} else {
		delta := time.Now().Sub(client.baseTime)
		timestamp = client.baseTimestamp + int32(delta/time.Second)
	}
	client.baseMutex.Unlock()
	return
}

func generateSecureRandomByteArray(length int) ([]byte, error) {
	result := make([]byte, length)
	_, err := rand.Read(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
