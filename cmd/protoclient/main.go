// Package protoclient provides a command line tool used to encrypt / decrypt
// and send / receive messages using the communication protocol.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/iot-bp-project-2018/raspi-server/internal/commproto"
	"github.com/iot-bp-project-2018/raspi-server/internal/mqttclient"
	"github.com/iot-bp-project-2018/raspi-server/internal/util/terminal"
	log "github.com/sirupsen/logrus"
)

var (
	configFlag  = flag.String("config", "", "load configuration from `file`")
	mqttFlag    = flag.String("mqtt", "tcp://192.168.10.1:1883", "MQTT broker URI (format is scheme://host:port)")
	verboseFlag = flag.Bool("verbose", false, "enable detailed logging")
)

var terminalMutex sync.Mutex
var terminalBuffer bytes.Buffer

func main() {
	flag.Parse()

	if *configFlag == "" {
		fmt.Fprintln(os.Stderr, "please specify a configuration file using the -config flag")
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

	ps := mqttclient.NewMQTTClientWithServer(*mqttFlag)
	client := commproto.NewClient(config, ps)

	client.RegisterCallback(func(sender string, datagramType commproto.DatagramType, encoding commproto.PayloadEncoding, data []byte) {
		if encoding == commproto.PayloadEncodingUTF8 {
			fmt.Printf("%s: %s\n", sender, string(data))
			return
		}
		fmt.Printf("%s: <encoded message> (encoding = %d)\n", sender, encoding)
	})

	input := make(chan string, 1)

	{
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-signals
			close(input)
		}()
	}

	terminalFd := int(syscall.Stdin)
	isTerminal := terminal.IsTerminal(terminalFd)
	var terminalState *terminal.State
	if isTerminal {
		var err error
		terminalState, err = terminal.MakeCbreak(terminalFd)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Debug("Failed to upgrade terminal")
		}
	}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Split(bufio.ScanRunes)
		for scanner.Scan() {
			terminalMutex.Lock()

			data := scanner.Bytes()

			if runtime.GOOS == "windows" {
				if isTerminal && len(data) == 1 && data[0] == '\r' {
					data[0] = '\n'
				}
			} else if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
				if isTerminal && len(data) == 1 && data[0] == '\x7f' {
					data[0] = '\b'
				}
			}

			if len(data) == 1 && data[0] == '\b' {
				os.Stdout.Write([]byte{'\b', ' ', '\b'})
			} else {
				os.Stdout.Write(data)
			}

			if len(data) == 1 && data[0] == '\n' {
				input <- terminalBuffer.String()
				terminalBuffer = bytes.Buffer{}
			} else if len(data) == 1 && data[0] == '\b' {
				if terminalBuffer.Len() > 0 {
					terminalBuffer.Truncate(terminalBuffer.Len() - 1)
				}
			} else {
				terminalBuffer.Write(data)
			}

			terminalMutex.Unlock()
		}
		close(input)
	}()

	in, out := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			terminalMutex.Lock()

			os.Stdout.Write(bytes.Repeat([]byte{'\r'}, terminalBuffer.Len()))
			os.Stdout.Write(bytes.Repeat([]byte{' '}, terminalBuffer.Len()))
			os.Stdout.Write(bytes.Repeat([]byte{'\r'}, terminalBuffer.Len()))
			os.Stdout.Write([]byte(scanner.Text()))
			os.Stdout.Write([]byte{'\n'})
			os.Stdout.Write(terminalBuffer.Bytes())

			terminalMutex.Unlock()
		}
	}()

	log.SetOutput(out)
	if terminal.IsTerminal(int(syscall.Stdout)) {
		terminal.EnableVirtualTerminalProcessing(int(syscall.Stdout))
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
	}

	client.Start()

	for line := range input {
		if line == "exit" {
			fmt.Fprintln(out, "bye")
			break
		}

		index := strings.Index(line, ":")
		if index == -1 {
			fmt.Fprintln(out, "to send messages use the format 'receiver: Text.'")
			continue
		}

		receiver, body := strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:])
		err := client.SendString(receiver, commproto.DatagramTypeMessage, body)
		if err != nil {
			fmt.Fprintln(out, "error:", err)
			continue
		}

		fmt.Fprintln(out, "ok")
	}

	if terminalState != nil {
		err = terminal.Restore(terminalFd, terminalState)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Debug("Failed to restore terminal")
		}
	}
}
