package main

import (
	"flag"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

var (
	wg     sync.WaitGroup
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

	client := newClient("connect", "connect", "Connect", false)

	wg.Add(1)
	timer := time.AfterFunc(1*time.Second, func() {
		wg.Done()
		assert.Fail("timeout")
	})

	client.AddCallback("001", func(e *irc.Event) {
		timer.Stop()
		defer wg.Done()

		assert.True(true)
	})

	defer client.Quit()
	go client.Loop()

	wg.Wait()
}

func TestRplWelcome(t *testing.T) {
	assert := assert.New(t)

	client := newClient("connect", "connect", "Connect", false)

	wg.Add(1)
	timer := time.AfterFunc(1*time.Second, func() {
		wg.Done()
		assert.Fail("timeout")
	})

	client.AddCallback("001", func(e *irc.Event) {
		timer.Stop()
		defer wg.Done()

		assert.Regexp(
			"Welcome to the .* Internet Relay Network .*!.*@.*$",
			e.Message(),
		)
	})

	defer client.Quit()
	go client.Loop()

	wg.Wait()
}

func TestUser_JOIN(t *testing.T) {
	assert := assert.New(t)

	wg.Add(1)
	timer := time.AfterFunc(1*time.Second, func() {
		wg.Done()
		assert.Fail("timeout")
	})

	client.AddCallback("353", func(e *irc.Event) {
		timer.Stop()
		defer wg.Done()

		assert.Equal(e.Arguments[0], "test")
		assert.Equal(e.Arguments[1], "=")
		assert.Equal(e.Arguments[2], "#test")
		assert.Equal(e.Arguments[3], "@test")
	})

	client.Join("#test")
	client.SendRaw("NAMES #test")
	wg.Wait()
}

func TestUser_PRIVMSG(t *testing.T) {
	assert := assert.New(t)

	wg.Add(1)
	timer := time.AfterFunc(1*time.Second, func() {
		wg.Done()
		assert.Fail("timeout")
	})

	clients["test1"].AddCallback("PRIVMSG", func(e *irc.Event) {
		timer.Stop()
		defer wg.Done()

		assert.Equal(e.Message(), "Hello World!")
		assert.Equal(e.User, "test")
	})

	client.Privmsg("test1", "Hello World!")
	wg.Wait()
}
