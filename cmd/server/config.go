package main

import "time"

var buttonTimeout = 5 * time.Second

const webserverEndpoint = ":80"
const mqttEndpoint = "localhost:1883"
const influxDBHost = "http://localhost:8086"

const influxDB = "bp"
const username = "bp"
const password = "xiedu2aNgaefie9Kaen1aivoochura"

const configDirectory = "config"
const networkFile = "config/network.json"
const tokenFile = "config/tokens.json"
