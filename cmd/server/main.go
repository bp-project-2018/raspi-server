// Package server provides the central executable of the project, responsible
// for receiving data from and sending commands to the sensors, storing the
// data, serving the webinterface and providing an API for it.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
)

func mqttHandler(sender string, datagramType commproto.DatagramType, encoding commproto.PayloadEncoding, data []byte) {
	if encoding != commproto.PayloadEncodingJSON {
		log.Printf("unknown payload encoding %d in message from %s\n", datagramType, sender)
		return
	}
	payload := SensorPayloadFromJSONBuffer(data)
	// Collect data
	fmt.Println(sender, payload)
	collectMetric("datapoint", Fields{"value": payload.Value}, Tags{"device": sender, "sensor": string(payload.SensorID), "type": payload.Type, "unit": payload.Unit})
}

func main() {
	os.MkdirAll(configDirectory, 0755)

	config, err := commproto.ParseConfiguration(networkFile)
	if err != nil {
		log.Println()
		log.Panicln(err)
	}

	ps := mqttclient.NewMQTTClientWithServer(mqttEndpoint)
	client := commproto.NewClient(config, ps)
	client.RegisterCallback(mqttHandler)
	client.Start()

	loadTokens()
	startWebserver()
}
