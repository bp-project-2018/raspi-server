package pubsubwrapper

import (
	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
)

type ReceiveCallback func(channel string, data []byte, callback commproto.PubSubCallback)
type PublishCallback func(channel string, data []byte, ps commproto.PubSubClient)

type Wrapper struct {
	ps        commproto.PubSubClient
	onReceive ReceiveCallback
	onPublish PublishCallback
}

func Wrap(ps commproto.PubSubClient, onReceive ReceiveCallback, onPublish PublishCallback) commproto.PubSubClient {
	if onReceive == nil {
		onReceive = func(channel string, data []byte, callback commproto.PubSubCallback) {
			callback(channel, data)
		}
	}
	if onPublish == nil {
		onPublish = func(channel string, data []byte, ps commproto.PubSubClient) {
			ps.Publish(channel, data)
		}
	}
	return &Wrapper{
		ps:        ps,
		onReceive: onReceive,
		onPublish: onPublish,
	}
}

func (w *Wrapper) Disconnect() {
	w.ps.Disconnect()
}

func (w *Wrapper) Subscribe(channel string, callback commproto.PubSubCallback) {
	w.ps.Subscribe(channel, func(channel string, data []byte) {
		w.onReceive(channel, data, callback)
	})
}

func (w *Wrapper) Unsubscribe(channel string) {
	w.ps.Unsubscribe(channel)
}

func (w *Wrapper) Publish(channel string, data []byte) {
	w.onPublish(channel, data, w.ps)
}
