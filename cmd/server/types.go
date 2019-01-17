package main

import (
	"encoding/json"
)

// SensorPayload contains all the fields sent by one sensor packet
type SensorPayload struct {
	SensorID byte    `json:"sensor_id"`
	Value    float32 `json:"value"`
	Type     string  `json:"type"`
	Unit     string  `json:"unit"`
}

// SensorPayloadFromJSONBuffer decodes a json byte array into SensorPayload
func SensorPayloadFromJSONBuffer(buffer []byte) SensorPayload {
	p := SensorPayload{}
	json.Unmarshal(buffer, &p)
	return p
}

// DataQueryRequest requests data from the database
type DataQueryRequest struct {
	DeviceID          string
	SensorID          string
	BeginUnix         int
	EndUnix           int
	ResolutionSeconds int
}
