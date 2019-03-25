// Package server provides the central executable of the project, responsible
// for receiving data from and sending commands to the sensors, storing the
// data, serving the webinterface and providing an API for it.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
	"github.com/iot-bp-project-2018/raspi-server/internal/testbuilder"
	log "github.com/sirupsen/logrus"
)

var (
	mqttFlag    = flag.String("mqtt", "tcp://localhost:1883", "MQTT broker URI (format is scheme://host:port)")
	testFlag    = flag.String("test", "", "Test server against a certain kind of attack (manipulation, delay, impersonation, injection, duplication)")
	verboseFlag = flag.Bool("verbose", false, "enable detailed logging")
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

// @Todo: Convert server package to logrus.
func main() {
	flag.Parse()

	if *verboseFlag {
		log.SetLevel(log.DebugLevel)
	}

	err := testbuilder.ValidateMode(*testFlag)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	os.MkdirAll(configDirectory, 0755)

	config, err := commproto.ParseConfiguration(networkFile)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	if *testFlag != "" {
		ps = testbuilder.Wrap(ps, *testFlag)
	}

	client := commproto.NewClient(config, ps)
	client.RegisterCallback(sensorDataHandler)
	client.Start()

	loadTokens()
	loadDevices()
	initMetrics()
	startWebserver()
}
