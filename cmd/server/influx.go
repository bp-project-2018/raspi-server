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

func queryMetrics(deviceID string, sensorID int, from, to time.Time, precisionSeconds int) [][]interface{} {
	cmd := fmt.Sprintf("select mean(value) from datapoint where time > %d and time < %d and device = '%s' and sensor = '%d' group by time(%ds)", from.UnixNano(), to.UnixNano(), deviceID, sensorID, precisionSeconds)
	q := client.Query{
		Command:  cmd,
		Database: influxDatabase,
	}
	resp, err := influxClient.Query(q)
	if err != nil {
		log.Println("[influx] data query failed:", err)
		return nil
	}
	if resp.Err != "" {
		log.Println("[influx] data query returned error:", resp.Err)
		return nil
	}
	if len(resp.Results) == 0 {
		log.Println("[influx] data query returned no results")
		return nil
	}
	result := resp.Results[0]
	if result.Err != "" {
		log.Println("[influx] data query result returned error:", result.Err)
		return nil
	}
	for i, message := range result.Messages {
		log.Printf("[influx] data query message %d (%s): %s", i+1, message.Level, message.Text)
	}
	if len(result.Series) == 0 {
		log.Println("[influx] data query returned no series")
		return nil
	}
	return result.Series[0].Values
}
