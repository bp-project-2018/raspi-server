package main

import "time"

var buttonTimeout = 5 * time.Second

const webserverEndpoint = ":80"
const mqttEndpoint = "localhost:1883"

const configDirectory = "config"
const networkFile = "config/network.json"
const tokenFile = "config/tokens.json"
