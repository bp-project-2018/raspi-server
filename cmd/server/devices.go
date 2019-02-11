package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"
)

// Device stores all information about a discovered device in the network
type Device struct {
	ID           string    `json:"id"`
	Sensors      []*Sensor `json:"sensors"`
	DiscoveredAt time.Time `json:"discoveredAt"`
}

// Sensor stores all information about a devices sensor
type Sensor struct {
	ID           byte      `json:"id"`
	Type         string    `json:"type"`
	Unit         string    `json:"unit"`
	DiscoveredAt time.Time `json:"discoveredAt"`
}

var deviceStorage struct {
	Devices []*Device `json:"devices"`
}

func addDevice(d *Device) {
	deviceStorage.Devices = append(deviceStorage.Devices, d)
	log.Printf("[devices] new device '%s' added to device storage\n", d.ID)
	saveDevices()
}

func loadDevices() {
	data, err := ioutil.ReadFile(devicesFile)
	if err != nil {
		deviceStorage.Devices = make([]*Device, 0)
		log.Println("[devices] empty devices list created")
	} else {
		json.Unmarshal(data, &deviceStorage)
		log.Printf("[devices] device list loaded with %d entries\n", len(deviceStorage.Devices))
	}
}

func saveDevices() {
	data, err := json.MarshalIndent(deviceStorage, "", "\t")
	if err != nil {
		log.Println("[devices] could not encode devices storage")
		log.Panicln(err)
	}
	err = ioutil.WriteFile(devicesFile, data, 0644)
	if err != nil {
		log.Println("[devices] failed to write devices file")
		log.Println(err)
	}
}

func getDevice(id string) *Device {
	for _, device := range deviceStorage.Devices {
		if device.ID == id {
			return device
		}
	}
	newDevice := &Device{ID: id, Sensors: []*Sensor{}, DiscoveredAt: time.Now()}
	addDevice(newDevice)
	return newDevice
}

func (d *Device) updateSensor(id byte, sensorType string, sensorUnit string) *Sensor {
	for _, sensor := range d.Sensors {
		if sensor.ID == id {
			return sensor
		}
	}
	newSensor := &Sensor{ID: id, Type: sensorType, Unit: sensorUnit, DiscoveredAt: time.Now()}
	d.Sensors = append(d.Sensors, newSensor)
	log.Printf("[devices] new sensor (%d, %s, %s) added to device '%s'\n", newSensor.ID, newSensor.Type, newSensor.Unit, d.ID)
	saveDevices()
	return newSensor
}
