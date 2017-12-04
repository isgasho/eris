package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

const (
	TIMEOUT = 3 * time.Second
)

var (
	server *eris.Server

	client  *irc.Connection
	clients map[string]*irc.Connection

	debug = flag.Bool("d", false, "enable debug logging")
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

	client.VerboseCallbackHandler = *debug
	client.Debug = *debug

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

	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	server = setupServer()

	wg := sync.WaitGroup{}
	wg.Add(3)

	client = newClient("test", "test", "Test", true)
	client.AddCallback("001", func(e *irc.Event) { wg.Done() })
	clients = make(map[string]*irc.Connection)
	clients["test1"] = newClient("test1", "test", "Test 1", true)
	clients["test1"].AddCallback("001", func(e *irc.Event) { wg.Done() })
	clients["test2"] = newClient("test2", "test", "Test 2", true)
	clients["test2"].AddCallback("001", func(e *irc.Event) { wg.Done() })

	wg.Wait()

	result := m.Run()

	client.Quit()
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
		actual <- true
	})

	defer client.Quit()
	go client.Loop()

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
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
		actual <- e.Message()
	})

	defer client.Quit()
	go client.Loop()

	select {
	case res := <-actual:
		assert.Regexp(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
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
		for i := range e.Arguments {
			actual <- e.Arguments[i]
		}
	})

	client.Join("#test")
	client.SendRaw("NAMES #test")

	for i := range expected {
		select {
		case res := <-actual:
			assert.Equal(expected[i], res)
		case <-time.After(TIMEOUT):
			assert.Fail("timeout")
		}
	}
}

func TestChannel_InviteOnly(t *testing.T) {
	assert := assert.New(t)

	var (
		expected bool
		actual   chan bool
	)

	expected = true
	actual = make(chan bool)

	clients["test1"].AddCallback("473", func(e *irc.Event) {
		actual <- true
	})
	clients["test1"].AddCallback("JOIN", func(e *irc.Event) {
		actual <- false
	})

	client.Join("#test2")
	client.Mode("#test2", "+i")
	time.Sleep(1 * time.Second)
	clients["test1"].Join("#test2")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
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
		actual <- e.Message()
	})

	client.Privmsg("test1", expected)

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_PRIVMSG(t *testing.T) {
	assert := assert.New(t)

	var (
		expected string
		actual   chan string
	)

	expected = "Hello World!"
	actual = make(chan string)

	client1 := newClient("client1", "client", "Client 1", false)
	client2 := newClient("client2", "client", "Client 2", false)

	client1.AddCallback("JOIN", func(e *irc.Event) {
		channel := e.Arguments[0]
		if channel == "#test3" {
			if e.Nick == "client1" {
				client1.SendRaw("INVITE client2 #test3")
			} else if e.Nick == "client2" {
				client1.Privmsg("#test3", expected)
			} else {
				assert.Fail(fmt.Sprintf("unexpected user %s joined %s", e.Nick, channel))
			}
		} else {
			assert.Fail(fmt.Sprintf("unexpected channel %s", channel))
		}
	})

	client2.AddCallback("INVITE", func(e *irc.Event) {
		client2.Join(e.Arguments[1])
	})
	client2.AddCallback("PRIVMSG", func(e *irc.Event) {
		actual <- e.Message()
	})

	defer client1.Quit()
	defer client2.Quit()
	go client1.Loop()
	go client2.Loop()

	client1.Join("#test3")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_NoExternal(t *testing.T) {
	assert := assert.New(t)

	var (
		expected bool
		actual   chan bool
	)

	expected = true
	actual = make(chan bool)

	client1 := newClient("client1", "client", "Client 1", false)
	client2 := newClient("client2", "client", "Client 2", false)

	client1.AddCallback("JOIN", func(e *irc.Event) {
		channel := e.Arguments[0]
		if channel == "#noexternal" {
			if e.Nick == "client1" {
				client2.Privmsg("#noexternal", "FooBar!")
			} else {
				assert.Fail(fmt.Sprintf("unexpected user %s joined %s", e.Nick, channel))
			}
		} else {
			assert.Fail(fmt.Sprintf("unexpected channel %s", channel))
		}
	})

	client.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Arguments[0] == "#noexternal" {
			actual <- false
		}
	})
	client2.AddCallback("404", func(e *irc.Event) {
		actual <- true
	})

	defer client1.Quit()
	defer client2.Quit()
	go client1.Loop()
	go client2.Loop()

	client1.Join("#noexternal")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}
