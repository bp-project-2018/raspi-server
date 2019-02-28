// Package pinger measures protocol latencies.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
	log "github.com/sirupsen/logrus"
)

var (
	configFlag  = flag.String("config", "", "load configuration from `file`")
	mqttFlag    = flag.String("mqtt", "tcp://192.168.10.1:1883", "MQTT broker URI (format is scheme://host:port)")
	verboseFlag = flag.Bool("verbose", false, "enable detailed logging")

	targetFlag = flag.String("target", "", "host address to which the ping messages should be sent")
	countFlag  = flag.Int("n", 100, "number of pings to send")
)

func main() {
	flag.Parse()

	if *configFlag == "" {
		fmt.Fprintln(os.Stderr, "please specify a configuration file using the -config flag")
		return
	}

	if *targetFlag == "" {
		fmt.Fprintln(os.Stderr, "please specifiy a target host using the -target flag")
		return
	}

	if *countFlag <= 0 {
		fmt.Fprintln(os.Stderr, "number of pings must be positive")
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

	target, count := *targetFlag, *countFlag
	pongChan := make(chan bool)
	durations := make([]time.Duration, 0)

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	client := commproto.NewClient(config, ps)

	client.RegisterCallback(func(sender string, data []byte) {
		if sender == target && string(data) == "pong" {
			pongChan <- true
		}
	})

	client.Start()

	startupPause := 3 * time.Second
	log.Printf("Waiting %v at startup\n", startupPause)
	time.Sleep(startupPause)
	log.Println("Starting measurement")

	for i := 0; i < count; i++ {
		iLog := log.WithFields(log.Fields{"iteration": i + 1})

		var start, end time.Time

		start = time.Now()
		client.SendString(target, "ping")
		select {
		case <-pongChan:
			end = time.Now()
		case <-time.After(2 * time.Second):
			iLog.Println("Ping timed out")
			continue
		}

		duration := end.Sub(start)
		durations = append(durations, duration)
		iLog.WithFields(log.Fields{"rtt": duration}).Println("Ping succeeded")
		time.Sleep(100 * time.Millisecond)
	}

	samples := len(durations)
	log.Printf("Succeeded: %d/%d", samples, count)

	if samples == 0 {
		return
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	log.Println("===== RTT stats =====")
	logStats(durations)

	for i, duration := range durations {
		durations[i] = duration / 2
	}

	log.Println("===== one-way stats =====")
	logStats(durations)

	log.Println("===== histogram =====")
	buckets := make([]int, 20)
	bucketSize := (durations[samples-1] + time.Millisecond) / time.Duration(len(buckets))
	bucketMax := 0
	for _, duration := range durations {
		index := int(duration / bucketSize)
		buckets[index]++
		if buckets[index] > bucketMax {
			bucketMax = buckets[index]
		}
	}
	for i, bucket := range buckets {
		log.Printf("%4dms %s", int(time.Duration(i+1)*bucketSize/time.Millisecond), strings.Repeat("*", bucket*80/bucketMax))
	}
}

func logStats(durations []time.Duration) {
	samples := len(durations)

	var sum, squareSum float64
	for _, duration := range durations {
		durationFloat := float64(duration/time.Nanosecond) * 1e-9
		sum += durationFloat
		squareSum += durationFloat * durationFloat
	}

	mean := sum / float64(samples)
	stdDev := math.Sqrt((squareSum - float64(samples)*mean*mean) / float64(samples-1))

	log.Printf("Mean:      %v", time.Duration(mean*1e9)*time.Nanosecond)
	log.Printf("Std. Dev.: %v", time.Duration(stdDev*1e9)*time.Nanosecond)

	median := durations[samples/2]
	min, max := durations[0], durations[samples-1]
	quantile25, quantile75 := durations[samples/4], durations[samples*3/4]

	log.Printf("Median:    %v", median)
	log.Printf("Min:       %v", min)
	log.Printf("Max:       %v", max)
	log.Printf("Quant. 25: %v", quantile25)
	log.Printf("Quant. 75: %v", quantile75)
}
