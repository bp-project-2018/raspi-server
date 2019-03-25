package testbuilder

import (
	"errors"
	"math/rand"
	"time"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/util/pubsubwrapper"
)

func ValidateMode(mode string) error {
	switch mode {
	case "manipulation", "delay", "impersonation", "injection", "duplication":
		return nil
	default:
		return errors.New("invalid test mode")
	}
}

func Wrap(ps commproto.PubSubClient, mode string) commproto.PubSubClient {
	switch mode {
	case "manipulation":
		return wrapManipulation(ps)
	case "delay":
		return wrapDelay(ps)
	case "impersonation":
		return wrapImpersonation(ps)
	case "injection":
		return wrapInjection(ps)
	case "duplication":
		return wrapDuplication(ps)
	default:
		return ps
	}
}

func wrapManipulation(ps commproto.PubSubClient) commproto.PubSubClient {
	return pubsubwrapper.Wrap(ps, func(channel string, data []byte, callback commproto.PubSubCallback) {
		prefixlen := 1 + int(data[0])
		// Check that the datagram is long enough.
		if len(data) > prefixlen+48 {
			// Flip one bit of the first data byte.
			data[prefixlen+24] = data[prefixlen+24] ^ 1
		}
		callback(channel, data)
	}, nil)
}

func wrapDelay(ps commproto.PubSubClient) commproto.PubSubClient {
	return pubsubwrapper.Wrap(ps, func(channel string, data []byte, callback commproto.PubSubCallback) {
		go func() {
			time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
			callback(channel, data)
		}()
	}, nil)
}

func wrapImpersonation(ps commproto.PubSubClient) commproto.PubSubClient {
	return pubsubwrapper.Wrap(ps, func(channel string, data []byte, callback commproto.PubSubCallback) {
		callback(channel, data)

		address, _ := commproto.ExtractAddress(data)
		iv, _ := commproto.GenerateSecureRandomByteArray(commproto.IVSize)
		timestamp := time.Now().UnixNano()
		payload := []byte("giant wheel of cheese")

		// The attacker does not know the following properties.
		key := make([]byte, commproto.KeySize)
		passphrase := ""

		// Create fake datagram using the address from the just received datagram.
		newdata := commproto.AssembleDatagram(address, iv, timestamp, payload, key, passphrase)
		callback(channel, newdata)
	}, nil)
}

func wrapInjection(ps commproto.PubSubClient) commproto.PubSubClient {
	return pubsubwrapper.Wrap(ps, func(channel string, data []byte, callback commproto.PubSubCallback) {
		callback(channel, data)

		address := "masterspy"
		iv, _ := commproto.GenerateSecureRandomByteArray(commproto.IVSize)
		timestamp := time.Now().UnixNano()
		payload := []byte("giant wheel of cheese")

		key := []byte{0x05, 0x5a, 0x27, 0x55, 0x97, 0x61, 0x0f, 0xaf, 0xcb, 0x3d, 0x43, 0x59, 0xbd, 0x79, 0x7b, 0x7b}
		passphrase := "secretsecret"

		// Inject custom datagram.
		newdata := commproto.AssembleDatagram(address, iv, timestamp, payload, key, passphrase)
		callback(channel, newdata)
	}, nil)
}

func wrapDuplication(ps commproto.PubSubClient) commproto.PubSubClient {
	return pubsubwrapper.Wrap(ps, func(channel string, data []byte, callback commproto.PubSubCallback) {
		// Deliver datagram twice.
		callback(channel, data)
		callback(channel, data)
	}, nil)
}
