package main

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

var (
	server *eris.Server

	client  *irc.Connection
	clients map[string]*irc.Connection

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

func newClient(nick, user, name string) *irc.Connection {
	client := irc.IRC(nick, user)
	client.RealName = name

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

	client = newClient("test", "test", "Test")
	clients = make(map[string]*irc.Connection)
	clients["test1"] = newClient("test1", "test", "Test 1")
	clients["test2"] = newClient("test2", "test", "Test 2")

	result := m.Run()

	for _, client := range clients {
		client.Quit()
	}

	server.Stop()

	os.Exit(result)
}

func TestConnection(t *testing.T) {
	client.AddCallback("001", func(e *irc.Event) {
		client.Quit()
	})
}

func TestConnection_RplWelcome(t *testing.T) {
	assert := assert.New(t)

	client.AddCallback("001", func(e *irc.Event) {
		defer client.Quit()
		assert.Regexp(
			"Welcome to the .* Internet Relay Chat Network$",
			e.Message(),
		)
	})
}

func TestConnection_User_PRIVMSG(t *testing.T) {
	assert := assert.New(t)

	clients["test1"].AddCallback("PRIVMSG", func(e *irc.Event) {
		assert.Equal(e.Message(), "Hello World!")
		assert.Equal(e.User, "test")
	})

	client.Privmsg("test1", "Hello World!")
}
