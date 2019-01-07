// Package server provides the central executable of the project, responsible
// for receiving data from and sending commands to the sensors, storing the
// data, serving the webinterface and providing an API for it.
package main

import (
	"log"
	"os"

	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
)

func valueHandler(channel string, data []byte) {
	p := SensorPayloadFromJSONBuffer(data)
	log.Println(channel, "->", p)
}

func main() {
	os.MkdirAll(configDirectory, 0755)

	client := mqttclient.NewMQTTClientWithServer(mqttEndpoint)
	client.Subscribe("master/inbox", valueHandler)

	loadTokens()
	startWebserver()
}
