// +build !linux !arm

package main

import (
	"context"
	"time"
)

// HardwareWaitForPairingButton waits until the pairing button is pressed or until the context expired
func HardwareWaitForPairingButton(c context.Context) bool {
	select {
	case <-time.After(5 * time.Second):
		return true
	case <-c.Done():
		return false
	}
}
