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

	sessionServer *sessionServer
	timeServer    *timeServer
	timeClient    *timeClient

	commandMutex sync.Mutex
	commands     map[string][]Payload

	callbacks []DatagramCallback
}

type DatagramCallback func(sender string, datagramType DatagramType, encoding PayloadEncoding, data []byte)

func NewClient(config *ClientConfiguration, ps PubSubClient) *Client {
	client := &Client{
		config: *config,
		ps:     ps,
	}
	if config.AcceptsCommands {
		client.sessionServer = &sessionServer{
			address:  config.HostAddress,
			partners: config.Partners,
			ps:       ps,
			sessions: make(map[string][]byte),
		}
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
	client.commands = make(map[string][]Payload)
	return client
}

func (client *Client) RegisterCallback(callback DatagramCallback) {
	if callback == nil {
		panic("nil callback")
	}
	client.callbacks = append(client.callbacks, callback)
}

func (client *Client) Start() {
	if client.sessionServer != nil {
		client.sessionServer.Start()
	}
	if client.timeServer != nil {
		client.timeServer.Start()
	}
	if client.timeClient != nil {
		client.timeClient.Start()
	}
	client.ps.Subscribe(fmt.Sprintf("%s/session", client.config.HostAddress), client.onSession)
	client.ps.Subscribe(fmt.Sprintf("%s/inbox", client.config.HostAddress), client.onDatagram)
}

func (client *Client) onSession(_ string, incomingDatagram []byte) {
	receiverAddress, sessionID, err := DisassembleSession(incomingDatagram)
	if err != nil {
		log.Info("Session client received invalid datagram")
		return
	}

	receiverConfig, configOk := client.config.Partners[receiverAddress]
	if !configOk {
		log.WithFields(log.Fields{"partner": receiverAddress}).Warn("Session client received session from unknown partner")
		return
	}

	var payload Payload
	var payloadOk, last bool

	client.commandMutex.Lock()
	outstanding := client.commands[receiverAddress]
	if len(outstanding) != 0 {
		payloadOk = true
		payload = outstanding[0]
		outstanding = outstanding[1:]
		if len(outstanding) == 0 {
			last = true
			delete(client.commands, receiverAddress)
		} else {
			client.commands[receiverAddress] = outstanding
		}
	}
	client.commandMutex.Unlock()

	if !payloadOk {
		log.Info("Session client received session, but has no commands to send")
		return
	}

	header := DatagramHeader{
		Type:          DatagramTypeCommand,
		Version:       0,
		Encoding:      payload.Encoding,
		SourceAddress: client.config.HostAddress,
	}

	iv, err := generateSecureRandomByteArray(16)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Session client failed to generate iv")
		return
	}

	datagram, err := AssembleDatagram(&header, sessionID, payload.Data, receiverConfig.Key, iv, receiverConfig.Passphrase)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Session client failed to assemble datagram")
		return
	}

	client.ps.Publish(fmt.Sprintf("%s/inbox", receiverAddress), datagram)

	if !last {
		client.ps.Publish(fmt.Sprintf("%s/session/request", receiverAddress), []byte(client.config.HostAddress))
	}
}

func (client *Client) onDatagram(_ string, datagram []byte) {
	header, err := ExtractPublicHeader(datagram)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Received invalid datagram")
		return
	}

	sender := header.SourceAddress
	senderConfig, ok := client.config.Partners[sender]
	if !ok {
		log.WithFields(log.Fields{"sender": sender}).Info("Ignoring datagram from unknown sender")
		return
	}

	switch header.Type {
	case DatagramTypeMessage:
		timestampData, data, err := DisassembleDatagram(datagram, header, 4, senderConfig.Key, senderConfig.Passphrase)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Warn("Received invalid datagram")
			return
		}

		timestamp := int32(0)
		timestamp += int32(timestampData[0]) << 24
		timestamp += int32(timestampData[1]) << 16
		timestamp += int32(timestampData[2]) << 8
		timestamp += int32(timestampData[3]) << 0

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
			callback(sender, header.Type, header.Encoding, data)
		}

	case DatagramTypeCommand:
		id, data, err := DisassembleDatagram(datagram, header, 16, senderConfig.Key, senderConfig.Passphrase)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Warn("Received invalid datagram")
			return
		}

		if client.sessionServer == nil {
			log.Warn("Received command, but no session server is running")
			return
		}

		ok := client.sessionServer.validateSessionID(sender, id)
		if !ok {
			log.Warn("Received invalid command datagram")
			return
		}

		// @Todo: @Sync: Protect client.callbacks??
		for _, callback := range client.callbacks {
			callback(sender, header.Type, header.Encoding, data)
		}
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
		timestamp, err := client.getTime()
		if err != nil {
			return fmt.Errorf("failed to get time: %v", err)
		}
		timestampData := []byte{
			byte(timestamp >> 24),
			byte(timestamp >> 16),
			byte(timestamp >> 8),
			byte(timestamp >> 0),
		}
		iv, err := generateSecureRandomByteArray(16)
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
		client.commandMutex.Lock()
		outstanding := client.commands[receiver]
		outstanding = append(outstanding, Payload{
			Encoding: encoding,
			Data:     data,
		})
		client.commands[receiver] = outstanding
		client.commandMutex.Unlock()

		if len(outstanding) == 1 {
			client.ps.Publish(fmt.Sprintf("%s/session/request", receiver), []byte(client.config.HostAddress))
		}

		return nil

	default:
		return fmt.Errorf("invalid datagram type: %d", datagramType)
	}
}

func (client *Client) getTime() (timestamp int32, err error) {
	if client.timeClient != nil {
		return client.timeClient.getTime()
	}
	return int32(time.Now().Unix()), nil
}

type sessionServer struct {
	address  string
	partners map[string]PartnerConfiguration
	ps       PubSubClient

	sessionMutex sync.Mutex
	sessions     map[string][]byte
}

func (server *sessionServer) Start() {
	log.WithFields(log.Fields{"addr": server.address}).Debug("Starting session server")
	server.ps.Subscribe(fmt.Sprintf("%s/session/request", server.address), server.onRequest)
}

func (server *sessionServer) onRequest(_ string, datagram []byte) {
	partnerAddress := string(datagram)
	if _, ok := server.partners[partnerAddress]; !ok {
		log.WithFields(log.Fields{"partner": partnerAddress}).Info("Session server received request from unknown partner")
		return
	}

	id, err := generateSecureRandomByteArray(16)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Session server failed to generate new session id")
		return
	}

	server.sessionMutex.Lock()
	server.sessions[partnerAddress] = id
	server.sessionMutex.Unlock()

	server.ps.Publish(fmt.Sprintf("%s/session", partnerAddress), AssembleSession(server.address, id))
}

func (server *sessionServer) validateSessionID(partnerAddress string, id []byte) bool {
	server.sessionMutex.Lock()
	defer server.sessionMutex.Unlock()

	stored := server.sessions[partnerAddress]
	if !bytes.Equal(stored, id) {
		return false
	}

	delete(server.sessions, partnerAddress)
	return true
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
