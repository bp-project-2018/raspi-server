package main

import (
	"context"
	"log"
	"time"

	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
)

const mqttHost = "localhost:1883"

func valueHandler(channel string, data []byte) {
	p := SensorPayloadFromJSONBuffer(data)
	log.Println(channel, "->", p)
}

func main() {
	log.Println("[main] waiting 5sec for button")
	timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	authorized := HardwareWaitForPairingButton(timeout)

	if !authorized {
		log.Println("[main] cancelling")
		return
	}
	log.Println("[main] we're good to go!")

	client := mqttclient.NewMQTTClientWithServer(mqttHost)
	client.Subscribe("master/inbox", valueHandler)

	for {
		time.Sleep(time.Second)
	}
}
