// Package protoclient provides a command line tool used to encrypt / decrypt
// and send / receive messages using the communication protocol.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
		fmt.Fprintln(os.Stderr, "please specify a configuration file using the -config flag")
		return
	}

	if *verboseFlag {
		log.SetLevel(log.DebugLevel)
	}

	config, err := commproto.ParseConfiguration(*configFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	interceptSignals()

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	client := commproto.NewClient(config, ps)

	client.RegisterCallback(func(sender string, datagramType commproto.DatagramType, encoding commproto.PayloadEncoding, data []byte) {
		if encoding == commproto.PayloadEncodingUTF8 {
			fmt.Printf("%s: %s\n", sender, string(data))
			return
		}
		fmt.Printf("%s: <encoded message> (encoding = %d)\n", sender, encoding)
	})

	client.Start()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if input == "exit" {
			fmt.Println("bye")
			os.Exit(0)
		}

		index := strings.Index(input, ":")
		if index == -1 {
			fmt.Println("to send messages use the format 'receiver: Text.'")
			continue
		}

		receiver, body := strings.TrimSpace(input[:index]), strings.TrimSpace(input[index+1:])
		err := client.SendString(receiver, commproto.DatagramTypeMessage, body)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}

		fmt.Println("ok")
	}
}

func interceptSignals() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		os.Exit(0)
	}()
}
