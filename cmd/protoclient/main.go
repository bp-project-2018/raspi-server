// protoclient is a command line tool used to encrypt / decrypt and send / receive messages using the communication protocol.
package main

import (
	"fmt"

	_ "github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	_ "github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
)

func main() {
	fmt.Println("Hello, Sailor!")
}
