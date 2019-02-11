// Package fakesensor reports fake sensor data using the communication protocol.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
	log "github.com/sirupsen/logrus"
)

var (
	configFlag  = flag.String("config", "", "load configuration from `file`")
	mqttFlag    = flag.String("mqtt", "tcp://192.168.10.1:1883", "MQTT broker URI (format is scheme://host:port)")
	verboseFlag = flag.Bool("verbose", false, "enable detailed logging")

	receiverFlag = flag.String("receiver", "kronos", "host address to which the data should be sent")

	brightnessFlag  = flag.Bool("brightness", false, "report brightness data")
	temperatureFlag = flag.Bool("temperature", false, "report temperature data")
	humidityFlag    = flag.Bool("humidity", false, "report humidity data")
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	flag.Parse()

	if *configFlag == "" {
		fmt.Fprintln(os.Stderr, "please specify a configuration file using the -config flag")
		return
	}

	if !*brightnessFlag && !*temperatureFlag && !*humidityFlag {
		fmt.Fprintln(os.Stderr, "please enable at least one measurement to report")
		fmt.Fprintln(os.Stderr, "e.g. -brightness -temperature -humidity")
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

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	client := commproto.NewClient(config, ps)
	client.Start()

	brightness := 100.0 * rand.Float64()
	temperature := 15.0 + 10.0*rand.Float64()
	humidity := 40.0 + 40.0*rand.Float64()

	for {
		time.Sleep(10 * time.Second)

		brightness += 5.0 * rand.NormFloat64()
		temperature += 0.5 * rand.NormFloat64()
		humidity += 1.2 * rand.NormFloat64()

		if brightness < 0.0 {
			brightness = 0.0
		} else if brightness > 100.0 {
			brightness = 100.0
		}

		if temperature < -20.0 {
			temperature = -20.0
		} else if temperature > 50.0 {
			temperature = 50.0
		}

		if humidity < 0.0 {
			humidity = 0.0
		} else if humidity > 100.0 {
			humidity = 100.0
		}

		if *brightnessFlag {
			measure(client, "brightness", brightness, "%")
		}

		if *temperatureFlag {
			measure(client, "temperature", temperature, "°C")
		}

		if *humidityFlag {
			measure(client, "humidity", humidity, "%")
		}
	}
}

type Measurement struct {
	DeviceID        string  `json:"device_id"`
	SensorID        int     `json:"sensor_id"`
	Value           float64 `json:"value"`
	MeasurementType string  `json:"type"`
	Unit            string  `json:"unit"`
}

func measure(client *commproto.Client, measurementType string, value float64, unit string) {
	measurement := Measurement{
		DeviceID:        "FFFF",
		SensorID:        1,
		Value:           value,
		MeasurementType: measurementType,
		Unit:            unit,
	}
	data, err := json.Marshal(measurement)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Failed to marshal measurement")
		return
	}
	err = client.Send(*receiverFlag, data)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Failed to send measurement")
		return
	}
	log.WithFields(log.Fields{"type": measurementType, "value": value, "unit": unit}).Info("Sent measurement")
}
