package commproto

// This file manages the flow of datagrams.

// PubSubClient provides an interface to a publish/subscribe system which is
// used by this package to send and receive the encrypted datagrams.
type PubSubClient interface {
	Disconnect()
	Subscribe(channel string, callback PubSubCallback)
	Unsubscribe(channel string)
	Publish(channel string, data []byte)
}

type PubSubCallback func(channel string, data []byte)
