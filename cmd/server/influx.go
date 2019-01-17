package main

import (
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
	go func() {
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
	}()
}
