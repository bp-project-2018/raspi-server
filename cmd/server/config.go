package main

import "time"

var buttonTimeout = 5 * time.Second

const webserverEndpoint = ":80"
const mqttEndpoint = "localhost:1883"

const influxHost = "http://localhost:8086"
const influxDatabase = "bp"
const influxUser = "bp"
const influxPassword = "xiedu2aNgaefie9Kaen1aivoochura"

const configDirectory = "config"
const networkFile = "config/network.json"
const tokenFile = "config/tokens.json"
