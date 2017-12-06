package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/renstrom/shortuuid"
	"github.com/stretchr/testify/assert"
	"github.com/thoj/go-ircevent"

	eris "github.com/prologic/eris/irc"
)

const (
	TIMEOUT = 3 * time.Second
)

var (
	server *eris.Server

	debug = flag.Bool("d", false, "enable debug logging")
)

func setupServer() *eris.Server {
	config := &eris.Config{}

	config.Network.Name = "Test"
	config.Server.Name = "test"
	config.Server.Description = "Test"
	config.Server.Listen = []string{":6667"}

	// SASL
	config.Account = map[string]*eris.PassConfig{
		"admin": &eris.PassConfig{"JDJhJDA0JGtUU1JVc1JOUy9DbEh1WEdvYVlMdGVnclp6YnA3NDBOZGY1WUZhdTZtRzVmb1VKdXQ5ckZD"},
	}

	server := eris.NewServer(config)

	go server.Run()

	return server
}

func randomValidName() string {
	var name eris.Name
	for {
		name = eris.NewName(shortuuid.New())
		if name.IsNickname() {
			break
		}
	}
	return name.String()
}

func newClient(start bool) *irc.Connection {
	name := randomValidName()
	client := irc.IRC(name, name)
	client.RealName = fmt.Sprintf("Test Client: %s", name)

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

	result := m.Run()

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

	client := newClient(false)

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

func TestSASL(t *testing.T) {
	assert := assert.New(t)

	expected := true
	actual := make(chan bool)

	client := newClient(false)
	client.SASLLogin = "admin"
	client.SASLPassword = "admin"

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

	client := newClient(false)

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

	actual = make(chan string)

	client := newClient(true)

	client.AddCallback("001", func(e *irc.Event) {
		expected = []string{e.Arguments[0], "=", "#join", fmt.Sprintf("@%s", e.Arguments[0])}
	})
	client.AddCallback("353", func(e *irc.Event) {
		for i := range e.Arguments {
			actual <- e.Arguments[i]
		}
	})

	defer client.Quit()

	client.Join("#join")
	client.SendRaw("NAMES #join")

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

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("324", func(e *irc.Event) {
		if strings.Contains(e.Arguments[2], "i") {
			client2.Join("#inviteonly")
		} else {
			client1.Mode("#inviteonly")
		}
	})

	client2.AddCallback("473", func(e *irc.Event) {
		actual <- true
	})
	client2.AddCallback("JOIN", func(e *irc.Event) {
		actual <- false
	})

	defer client1.Quit()
	defer client2.Quit()

	client1.Join("#inviteonly")
	client1.Mode("#inviteonly", "+i")
	client1.Mode("#inviteonly")

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

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("001", func(e *irc.Event) {
		client1.Privmsg(client2.GetNick(), expected)

	})
	client1.AddCallback("PRIVMSG", func(e *irc.Event) {
		actual <- e.Message()
	})

	client2.AddCallback("001", func(e *irc.Event) {
		client2.Privmsg(client1.GetNick(), expected)
	})
	client2.AddCallback("PRIVMSG", func(e *irc.Event) {
		actual <- e.Message()
	})

	defer client1.Quit()
	defer client2.Quit()

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

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("JOIN", func(e *irc.Event) {
		client1.Privmsg(e.Arguments[0], expected)
	})
	client2.AddCallback("JOIN", func(e *irc.Event) {
		client2.Privmsg(e.Arguments[0], expected)
	})

	client1.AddCallback("PRIVMSG", func(e *irc.Event) {
		actual <- e.Message()
	})
	client2.AddCallback("PRIVMSG", func(e *irc.Event) {
		actual <- e.Message()
	})

	defer client1.Quit()
	defer client2.Quit()

	client1.Join("#channelprivmsg")
	client2.Join("#channelprivmsg")

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

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("JOIN", func(e *irc.Event) {
		channel := e.Arguments[0]
		if channel == "#noexternal" {
			if e.Nick == client1.GetNick() {
				client2.Privmsg("#noexternal", "FooBar!")
			} else {
				assert.Fail(fmt.Sprintf("unexpected user %s joined %s", e.Nick, channel))
			}
		} else {
			assert.Fail(fmt.Sprintf("unexpected channel %s", channel))
		}
	})

	client2.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Arguments[0] == "#noexternal" {
			actual <- false
		}
	})
	client2.AddCallback("404", func(e *irc.Event) {
		actual <- true
	})

	defer client1.Quit()
	defer client2.Quit()

	client1.Join("#noexternal")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_SetTopic_InvalidChannel(t *testing.T) {
	assert := assert.New(t)

	expected := true
	actual := make(chan bool)

	client1 := newClient(true)
	defer client1.Quit()

	client1.AddCallback("403", func(e *irc.Event) {
		actual <- true
	})

	client1.SendRaw("TOPIC #invalidchannel :FooBar")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_SetTopic_NotOnChannel(t *testing.T) {
	assert := assert.New(t)

	expected := true
	actual := make(chan bool)

	client1 := newClient(true)
	client2 := newClient(true)
	defer client1.Quit()
	defer client2.Quit()

	client1.AddCallback("442", func(e *irc.Event) {
		actual <- true
	})
	client2.AddCallback("JOIN", func(e *irc.Event) {
		client1.SendRaw("TOPIC #notonchannel :FooBar")
	})

	client2.Join("#notonchannel")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_BadChannelKey(t *testing.T) {
	assert := assert.New(t)

	var (
		expected bool
		actual   chan bool
	)

	expected = true
	actual = make(chan bool)

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("324", func(e *irc.Event) {
		if strings.Contains(e.Arguments[2], "k") {
			client2.Join(e.Arguments[1])
		} else {
			client1.Mode("#badchannelkey")
		}
	})

	client2.AddCallback("JOIN", func(e *irc.Event) {
		if e.Nick == client2.GetNick() && e.Arguments[0] == "#badchannelkey" {
			actual <- false
		}
	})
	client2.AddCallback("475", func(e *irc.Event) {
		actual <- true
	})

	defer client1.Quit()
	defer client2.Quit()

	client1.Join("#badchannelkey")
	client1.Mode("#badchannelkey", "+k", "opensesame")
	client1.Mode("#badchannelkey")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}

func TestChannel_GoodChannelKey(t *testing.T) {
	assert := assert.New(t)

	var (
		expected bool
		actual   chan bool
	)

	expected = true
	actual = make(chan bool)

	client1 := newClient(true)
	client2 := newClient(true)

	client1.AddCallback("324", func(e *irc.Event) {
		if strings.Contains(e.Arguments[2], "k") {
			client2.SendRawf("JOIN %s :opensesame", e.Arguments[1])
		} else {
			client1.Mode("#goodchannelkey")
		}
	})

	client2.AddCallback("JOIN", func(e *irc.Event) {
		if e.Nick == client2.GetNick() && e.Arguments[0] == "#goodchannelkey" {
			actual <- true
		}
	})
	client2.AddCallback("475", func(e *irc.Event) {
		actual <- false
	})

	defer client1.Quit()
	defer client2.Quit()

	client1.Join("#goodchannelkey")
	client1.Mode("#goodchannelkey", "+k", "opensesame")
	client1.Mode("#goodchannelkey")

	select {
	case res := <-actual:
		assert.Equal(expected, res)
	case <-time.After(TIMEOUT):
		assert.Fail("timeout")
	}
}
