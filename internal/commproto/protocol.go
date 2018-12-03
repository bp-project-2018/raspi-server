package commproto

type PubSubCallback func(channel string, data []byte)

type PubSubClient interface {
	Subscribe(channel string, callback PubSubCallback)
	Unsubscribe(channel string)
	Publish(channel string, data []byte)
}
