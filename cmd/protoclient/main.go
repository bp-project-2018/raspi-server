// Package protoclient provides a command line tool used to encrypt / decrypt
// and send / receive messages using the communication protocol.
package main

import (
	"flag"
	"fmt"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
	log "github.com/sirupsen/logrus"
)

var (
	configFlag  = flag.String("config", "", "load configuration from `file`")
	mqttFlag    = flag.String("mqtt", ":1883", "MQTT broker URI (format is scheme://host:port)")
	verboseFlag = flag.Bool("verbose", false, "enable detailed logging")
)

func main() {
	flag.Parse()

	if *configFlag == "" {
		fmt.Println("please specify a configuration file using the -config flag")
		return
	}

	if *verboseFlag {
		log.SetLevel(log.DebugLevel)
	}

	config, err := commproto.ParseConfiguration(*configFlag)
	if err != nil {
		fmt.Println(err)
		return
	}

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	client := commproto.NewClient(config, ps)

	client.Start()

	// Block indefinitely.
	select {}
}
