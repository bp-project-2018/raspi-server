package commproto

// This file manages the flow of datagrams.

import (
	"crypto/rand"
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
	config     ClientConfiguration
	ps         PubSubClient
	timeServer *timeServer
	timeClient *timeClient
}

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

func (client *Client) Start() {
	if client.timeServer != nil {
		client.timeServer.Start()
	}
	if client.timeClient != nil {
		client.timeClient.Start()
	}
}

func (client *Client) SendString(receiver string, datagramType DatagramType, data string) error {
	return client.Send(receiver, datagramType, PayloadEncodingUTF8, []byte(data))
}

func (client *Client) SendBytes(receiver string, datagramType DatagramType, data []byte) error {
	return client.Send(receiver, datagramType, PayloadEncodingBinary, data)
}

func (client *Client) Send(receiver string, datagramType DatagramType, encoding PayloadEncoding, data []byte) error {
	receiverConfig, ok := client.config.Partners[receiver]
	if !ok {
		return fmt.Errorf("unknown receiver: %s", receiver)
	}

	switch datagramType {
	case DatagramTypeMessage:
		header := DatagramHeader{
			Type:          datagramType,
			Version:       0,
			Encoding:      encoding,
			SourceAddress: client.config.HostAddress,
		}
		timestamp := int32(time.Now().Unix())
		if client.timeClient != nil {
			timestamp = client.timeClient.getTime()
		}
		timestampData := []byte{
			byte(timestamp >> 24),
			byte(timestamp >> 16),
			byte(timestamp >> 8),
			byte(timestamp >> 0),
		}
		iv, err := generateIV()
		if err != nil {
			return fmt.Errorf("failed to generate iv: %v", err)
		}
		datagram, err := AssembleDatagram(&header, timestampData, data, receiverConfig.Key, iv, receiverConfig.Passphrase)
		if err != nil {
			return fmt.Errorf("failed to assemble datagram: %v", err)
		}
		client.ps.Publish(fmt.Sprintf("%s/inbox", receiver), datagram)
		return nil

	case DatagramTypeCommand:
		panic("command not implemented yet")

	default:
		return fmt.Errorf("invalid datagram type: %d", datagramType)
	}
}

func generateIV() ([]byte, error) {
	iv := make([]byte, 16)
	_, err := rand.Read(iv)
	if err != nil {
		return nil, err
	}
	return iv, nil
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

func (client *timeClient) getTime() (timestamp int32) {
	client.baseMutex.Lock()
	if client.baseTime.IsZero() {
		// Time not set. What should we do?
	} else {
		delta := time.Now().Sub(client.baseTime)
		timestamp = client.baseTimestamp + int32(delta/time.Second)
	}
	client.baseMutex.Unlock()
	return
}
