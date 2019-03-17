// Package mqttclient provides an MQTT-based implentation of a PubSubClient
// for the commproto package.
package mqttclient

import (
	"fmt"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/sirupsen/logrus"
)

var log = logrus.StandardLogger().WithFields(logrus.Fields{"package": "mqttclient"})

type subscription struct {
	channel  string
	callback commproto.PubSubCallback
}

type mqttClient struct {
	client mqtt.Client

	// mutex protects subscriptions and connected.
	mutex         sync.Mutex
	subscriptions []subscription
	connected     bool
}

// NewMQTTClientWithServer configures a new MQTT client using the specified
// server and a client ID generated from the hostname.
func NewMQTTClientWithServer(server string) commproto.PubSubClient {
	options := mqtt.NewClientOptions()
	options.AddBroker(server)
	options.SetClientID(getClientID())
	options.SetConnectTimeout(1 * time.Second)
	return NewMQTTClientWithOptions(options)
}

func getClientID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s%d", hostname, time.Now().Unix())
}

// NewMQTTClientWithOptions configures a new MQTT client using the provided
// options.
func NewMQTTClientWithOptions(options *mqtt.ClientOptions) commproto.PubSubClient {
	c := new(mqttClient)

	if options.OnConnect != nil {
		customOnConnect := options.OnConnect
		options.OnConnect = func(client mqtt.Client) {
			go customOnConnect(client)
			c.onConnect()
		}
	} else {
		options.OnConnect = func(client mqtt.Client) {
			c.onConnect()
		}
	}

	if options.OnConnectionLost != nil {
		customOnConnectionLost := options.OnConnectionLost
		options.OnConnectionLost = func(client mqtt.Client, err error) {
			go customOnConnectionLost(client, err)
			c.onConnectionLost(err)
		}
	} else {
		options.OnConnectionLost = func(client mqtt.Client, err error) {
			c.onConnectionLost(err)
		}
	}

	c.client = mqtt.NewClient(options)
	go c.connect()
	return c
}

func (c *mqttClient) Disconnect() {
	c.client.Disconnect(250) // ms
}

func (c *mqttClient) Subscribe(channel string, callback commproto.PubSubCallback) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sub := subscription{channel: channel, callback: callback}
	c.subscriptions = append(c.subscriptions, sub)

	if c.connected {
		c.subscribeTo(sub)
	}
}

func (c *mqttClient) Unsubscribe(channel string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := 0; i < len(c.subscriptions); i++ {
		if channel == c.subscriptions[i].channel {
			c.subscriptions[i] = c.subscriptions[len(c.subscriptions)-1]
			c.subscriptions = c.subscriptions[:len(c.subscriptions)-1]
			i--
		}
	}

	if c.connected {
		// Ignores the returned token for now...
		c.client.Unsubscribe(channel)
	}
}

func (c *mqttClient) Publish(channel string, data []byte) {
	// Ignores the returned token for now...
	c.client.Publish(channel, 0, false, data)
}

func (c *mqttClient) connect() {
	for {
		reader := c.client.OptionsReader()
		log.WithFields(logrus.Fields{"clientID": reader.ClientID()}).Debug("Trying to connect")
		token := c.client.Connect()
		if !token.Wait() || token.Error() == nil {
			log.Println("Connection acquired")
			return
		}
		message := token.Error().Error()
		log.Warnln("Connect error:", message)
		time.Sleep(10 * time.Second)
	}
}

func (c *mqttClient) onConnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, sub := range c.subscriptions {
		c.subscribeTo(sub)
	}

	c.connected = true
}

func (c *mqttClient) subscribeTo(sub subscription) {
	// Ignores the returned token for now...
	c.client.Subscribe(sub.channel, 0, func(client mqtt.Client, message mqtt.Message) {
		sub.callback(message.Topic(), message.Payload())
	})
}

func (c *mqttClient) onConnectionLost(err error) {
	c.mutex.Lock()
	c.connected = false
	c.mutex.Unlock()

	log.Println("Connection lost:", err)
	time.Sleep(10 * time.Second)
	c.connect()
}
