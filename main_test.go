package main

import (
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

var (
	done   chan bool
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

func newClient(nick, user, name string, start bool) *irc.Connection {
	client := irc.IRC(nick, user)
	client.RealName = name

	err := client.Connect("localhost:6667")
	if err != nil {
		log.Fatalf("error setting up test client: %s", err)
	}

	if start {
		go client.Loop()
	}

	return client
}

func TestMain(m *testing.M) {
	flag.Parse()

	done = make(chan bool)

	server = setupServer()

	client = newClient("test", "test", "Test", true)
	clients = make(map[string]*irc.Connection)
	clients["test1"] = newClient("test1", "test", "Test 1", true)
	clients["test2"] = newClient("test2", "test", "Test 2", true)

	result := m.Run()

	for _, client := range clients {
		client.Quit()
	}

	server.Stop()

	os.Exit(result)
}

func TestConnection(t *testing.T) {
	assert := assert.New(t)

	var (
		expected bool
		actual   chan bool
	)

	expected = true
	actual = make(chan bool)

	client := newClient("connect", "connect", "Connect", false)

	client.AddCallback("001", func(e *irc.Event) {
		defer func() { done <- true }()

		actual <- true
	})

	time.AfterFunc(1*time.Second, func() { done <- true })
	defer client.Quit()
	go client.Loop()
	<-done

	assert.Equal(expected, <-actual)
}

func TestRplWelcome(t *testing.T) {
	assert := assert.New(t)

	var (
		expected string
		actual   chan string
	)

	expected = "Welcome to the .* Internet Relay Network .*!.*@.*"
	actual = make(chan string)

	client := newClient("connect", "connect", "Connect", false)

	client.AddCallback("001", func(e *irc.Event) {
		defer func() { done <- true }()

		actual <- e.Message()
	})

	time.AfterFunc(1*time.Second, func() { done <- true })
	defer client.Quit()
	go client.Loop()
	<-done

	assert.Regexp(expected, <-actual)
}

func TestUser_JOIN(t *testing.T) {
	assert := assert.New(t)

	var (
		expected []string
		actual   chan string
	)

	expected = []string{"test", "=", "#test", "@test"}
	actual = make(chan string)

	client.AddCallback("353", func(e *irc.Event) {
		defer func() { done <- true }()

		for i := range e.Arguments {
			actual <- e.Arguments[i]
		}
	})

	time.AfterFunc(1*time.Second, func() { done <- true })
	client.Join("#test")
	client.SendRaw("NAMES #test")
	<-done

	for i := range expected {
		assert.Equal(expected[i], <-actual)
	}
}

func TestUser_PRIVMSG(t *testing.T) {
	assert := assert.New(t)

	var (
		expected string
		actual   chan string
	)

	expected = "Hello World!"
	actual = make(chan string)

	clients["test1"].AddCallback("PRIVMSG", func(e *irc.Event) {
		defer func() { done <- true }()

		actual <- e.Message()
	})

	time.AfterFunc(1*time.Second, func() { done <- true })
	client.Privmsg("test1", expected)
	<-done

	assert.Equal(expected, <-actual)
}
