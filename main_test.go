package main

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

var (
	client *irc.Connection
	server *eris.Server

	tls = flag.Bool("tls", false, "run tests with TLS")
)

func setupServer() *eris.Server {
	config := &eris.Config{}

	config.Network.Name = "Test"
	config.Server.Name = "test"
	config.Server.Description = "Test"
	config.Server.Listen = []string{":6667"}

	server := eris.NewServer(config)

	go server.Run()

	return server
}

func setupClient() *irc.Connection {
	client := irc.IRC("test", "test")
	client.RealName = "Test"

	err := client.Connect("localhost:6667")
	if err != nil {
		log.Fatalf("error setting up test client: %s", err)
	}

	go client.Loop()

	return client
}

func TestMain(m *testing.M) {
	flag.Parse()

	server = setupServer()
	client = setupClient()

	result := m.Run()

	client.Quit()
	server.Stop()

	os.Exit(result)
}

func TestConnection(t *testing.T) {
	client.AddCallback("001", func(e *irc.Event) {
		client.Quit()
	})
}
