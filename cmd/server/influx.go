package main

import (
	"fmt"
	"log"
	"time"

	"github.com/influxdata/influxdb1-client/v2"
)

var influxClient client.Client

func initMetrics() {
	client, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     influxHost,
		Username: influxUser,
		Password: influxPassword,
	})
	if err != nil {
		panic(err)
	}
	influxClient = client
}

// Fields ...
type Fields map[string]interface{}

// Tags ...
type Tags map[string]string

func collectMetric(eventType string, fields Fields, tags Tags) {
	if influxClient == nil {
		return
	}
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  influxDatabase,
		Precision: "ns",
	})
	if err != nil {
		log.Fatalln("error: ", err)
	}
	pt, err := client.NewPoint(eventType, tags, fields, time.Now())
	if err != nil {
		log.Fatalln("error: ", err)
	}
	bp.AddPoint(pt)
	influxClient.Write(bp)
}

func queryMetrics(deviceID, sensorID string, from, to time.Time, precisionSeconds int) [][]interface{} {
	cmd := fmt.Sprintf("select mean(value) from datapoint where time > %d and time < %d and device = '%s' and sensor = '%s' group by time(%ds)", from.UnixNano(), to.UnixNano(), deviceID, sensorID, precisionSeconds)
	q := client.Query{
		Command:  cmd,
		Database: influxDatabase,
	}

	resp, err := influxClient.Query(q)

	if err != nil {
		log.Println("[influx] data query failed:", err)
		return nil
	}

	return resp.Results[0].Series[0].Values
}
