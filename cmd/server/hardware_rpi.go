// +build linux,arm

package main

import (
	"context"
	"log"
	"time"

	"github.com/stianeikeland/go-rpio"
)

const (
	// Note(Tobias): rpio uses the raw BCM2835 pinouts.
	// Refer to the rpio documentation for a mapping between physical and BCM pin numbers.
	// Raspberry Pi pinout map: https://de.pinout.xyz
	// Rpio translation table:  https://godoc.org/github.com/stianeikeland/go-rpio
	pinButtonPair = rpio.Pin(21) // physical pin 40
)

var hardwareOpened = false

// OpenHardware initializes the hardware
func openHardware() {
	hardwareOpened = true
	err := rpio.Open()
	if err != nil {
		panic(err)
	}
	pinButtonPair.Input()
	pinButtonPair.PullUp()
}

// CloseHardware closes the hardware handles
func closeHardware() {
	err := rpio.Close()
	if err != nil {
		log.Fatalln("[hardware] Failed to close rpio:", err)
	}
	hardwareOpened = false
}

// HardwareWaitForPairingButton waits until the pairing button is pressed or until the context expired
func HardwareWaitForPairingButton(c context.Context) bool {
	if hardwareOpened {
		log.Println("[hardware] Blocked double pairing button wait call")
		return false
	}
	openHardware()
	defer closeHardware()
	for {
		if pinButtonPair.Read() == rpio.Low {
			return true
		}
		select {
		case <-c.Done():
			return false
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}
