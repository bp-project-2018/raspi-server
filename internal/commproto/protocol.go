package commproto

// This file manages the flow of datagrams.

import (
	"bytes"
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

	timeClient *timeClient

	callbacks []DatagramCallback
}

type DatagramCallback func(sender string, data []byte)

func NewClient(config *ClientConfiguration, ps PubSubClient) *Client {
	client := &Client{
		config: *config,
		ps:     ps,
	}
	if serverAddress := config.UseTimeServer; serverAddress != "" {
		serverConfig, ok := config.Partners[serverAddress]
		if !ok {
			panic("time server address not in 'partners'")
		}
		client.timeClient = &timeClient{
			clientAddress:    config.HostAddress,
			serverAddress:    serverAddress,
			serverPassphrase: serverConfig.Passphrase,
			ps:               ps,
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
	if client.config.HostTimeServer {
		log.Debug("Starting time server")
		client.ps.Subscribe(fmt.Sprintf("%s/time/request", client.config.HostAddress), client.onTimeRequest)
	}
	if client.timeClient != nil {
		client.timeClient.Start()
	}
	client.ps.Subscribe(fmt.Sprintf("%s/inbox", client.config.HostAddress), client.onDatagram)
}

func (client *Client) onTimeRequest(channel string, request []byte) {
	// @Todo: @Security: Maybe we should start dropping requests, if they come in too fast, to prevent a DOS attack.

	partner, ok := ExtractAddress(request)
	if !ok {
		log.Warn("Time server received invalid message")
		return
	}

	partnerConfig, ok := client.config.Partners[partner]
	if !ok {
		log.WithFields(log.Fields{"sender": partner}).Info("Ignoring time request from unknown sender")
		return
	}

	nonce, err := DisassembleTimeRequest(request, partner, partnerConfig.Passphrase)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Received invalid time request")
		return
	}

	timestamp := time.Now().UnixNano()
	response := AssembleTimeResponse(client.config.HostAddress, timestamp, nonce, partnerConfig.Passphrase)
	client.ps.Publish(fmt.Sprintf("%s/time", partner), response)
	log.WithFields(log.Fields{"receiver": partner, "timestamp": timestamp}).Debug("Time server sent time")
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

	timestamp, data, err := DisassembleDatagram(datagram, sender, senderConfig.Key, senderConfig.Passphrase)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Received invalid datagram")
		return
	}

	current, err := client.getTime()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Failed to ge time while receiving datagram")
		return
	}

	if delta := timestamp - current; delta < -1000000000 /* ns */ || delta > 1000000000 /* ns */ { // @Hardcoded
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
	iv, err := generateSecureRandomByteArray(IVSize)
	if err != nil {
		return fmt.Errorf("failed to generate iv: %v", err)
	}
	datagram := AssembleDatagram(client.config.HostAddress, iv, timestamp, data, receiverConfig.Key, receiverConfig.Passphrase)
	client.ps.Publish(fmt.Sprintf("%s/inbox", receiver), datagram)
	return nil
}

func (client *Client) getTime() (timestamp int64, err error) {
	if client.timeClient != nil {
		return client.timeClient.getTime()
	}
	return time.Now().UnixNano(), nil
}

type timeClient struct {
	clientAddress    string
	serverAddress    string
	serverPassphrase string
	ps               PubSubClient

	mutex sync.Mutex
	// The last nonce had the value lastNonce and was sent at local time lastTime.
	lastNonce []byte
	lastTime  time.Time
	// The time server reported baseTimestamp at local time baseTime.
	baseTimestamp int64
	baseTime      time.Time
}

func (client *timeClient) Start() {
	log.WithFields(log.Fields{"server-addr": client.serverAddress}).Debug("Starting time client")
	client.ps.Subscribe(fmt.Sprintf("%s/time", client.clientAddress), client.onTimeResponse)
	client.publishRequest()
	go client.requestLoop()
}

func (client *timeClient) onTimeResponse(channel string, response []byte) {
	sender, ok := ExtractAddress(response)
	if !ok {
		log.Warn("Time client received invalid time response")
		return
	}
	if sender != client.serverAddress {
		log.WithFields(log.Fields{"sender": sender}).Warn("Time client received time response from unkown time server")
		return
	}
	timestamp, nonce, err := DisassembleTimeResponse(response, sender, client.serverPassphrase)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Info("Time client received invalid time response")
		return
	}

	client.mutex.Lock()
	defer client.mutex.Unlock()

	if client.lastNonce == nil || !bytes.Equal(nonce, client.lastNonce) {
		log.Warn("Time client received invalid time response")
		return
	}

	now := time.Now()
	diff := now.Sub(client.lastTime)

	client.lastNonce = nil
	client.lastTime = time.Time{}

	if diff > 100*time.Millisecond {
		log.Warn("Time client received outdated time response")
		return
	}

	client.baseTimestamp = timestamp
	client.baseTime = now

	log.WithFields(log.Fields{"addr": client.serverAddress, "timestamp": timestamp}).Debug("Time client received time")
}

func (client *timeClient) publishRequest() {
	nonce, err := generateSecureRandomByteArray(NonceSize)
	if err != nil {
		log.WithFields(log.Fields{"addr": client.serverAddress, "err": err}).Warn("Time client failed to generate nonce")
		return
	}

	request := AssembleTimeRequest(client.clientAddress, nonce, client.serverPassphrase)

	client.mutex.Lock()
	client.lastNonce = nonce
	client.lastTime = time.Now()
	client.mutex.Unlock()

	log.WithFields(log.Fields{"server-addr": client.serverAddress}).Debug("Time client will send request")
	client.ps.Publish(fmt.Sprintf("%s/time/request", client.serverAddress), request)
}

func (client *timeClient) requestLoop() {
	for {
		time.Sleep(time.Second)
		client.mutex.Lock()
		baseTime := client.baseTime
		client.mutex.Unlock()
		if !baseTime.IsZero() {
			return // time received, all is well :)
		}
		client.publishRequest()
	}
}

func (client *timeClient) getTime() (timestamp int64, err error) {
	client.mutex.Lock()
	if client.baseTime.IsZero() {
		err = errors.New("no time server connection")
	} else {
		delta := time.Now().Sub(client.baseTime)
		timestamp = client.baseTimestamp + int64(delta/time.Nanosecond)
	}
	client.mutex.Unlock()
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
