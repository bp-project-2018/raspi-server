// Package protoclient provides a command line tool used to encrypt / decrypt
// and send / receive messages using the communication protocol.
package main

import (
	"flag"
	"fmt"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	_ "github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
)

var (
	configFlag = flag.String("config", "", "load configuration from `file`")
)

func main() {
	flag.Parse()

	if *configFlag == "" {
		fmt.Println("please specify a configuration file using the -config flag")
		return
	}

	config, err := commproto.ParseConfiguration(*configFlag)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Hello, Sailor!")
	fmt.Println(config)
}
