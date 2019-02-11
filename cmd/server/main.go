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

func sensorDataHandler(sender string, data []byte) {
	payload := SensorPayloadFromJSONBuffer(data)
	// Collect data
	fmt.Println(sender, payload)
	sensorIDStr := fmt.Sprintf("%d", payload.SensorID)
	collectMetric("datapoint", Fields{"value": payload.Value}, Tags{"device": sender, "sensor": sensorIDStr, "type": payload.Type, "unit": payload.Unit})
	// Update device and sensor in device cache if necessary
	d := getDevice(sender)
	d.updateSensor(payload.SensorID, payload.Type, payload.Unit)
}

func main() {
	os.MkdirAll(configDirectory, 0755)

	config, err := commproto.ParseConfiguration(networkFile)
	if err != nil {
		log.Panicln(err)
	}

	ps := mqttclient.NewMQTTClientWithServer(mqttEndpoint)
	client := commproto.NewClient(config, ps)
	client.RegisterCallback(sensorDataHandler)
	client.Start()

	loadTokens()
	loadDevices()
	initMetrics()
	startWebserver()
}
