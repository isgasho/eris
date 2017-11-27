package irc

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	IDLE_TIMEOUT = time.Minute // how long before a client is considered idle
	QUIT_TIMEOUT = time.Minute // how long after idle before a client is kicked
)

type Client struct {
	atime        time.Time
	authorized   bool
	awayMessage  Text
	capabilities CapabilitySet
	capState     CapState
	channels     *ChannelSet
	ctime        time.Time
	flags        map[UserMode]bool
	hasQuit      bool
	hops         uint
	hostname     Name
	hostmask     Name // Cloacked hostname (SHA256)
	pingTime     time.Time
	idleTimer    *time.Timer
	nick         Name
	quitTimer    *time.Timer
	realname     Text
	registered   bool
	sasl         *SaslState
	server       *Server
	socket       *Socket
	replies      chan string
	username     Name
}

func NewClient(server *Server, conn net.Conn) *Client {
	now := time.Now()
	client := &Client{
		atime:        now,
		authorized:   len(server.password) == 0,
		capState:     CapNone,
		capabilities: make(CapabilitySet),
		channels:     NewChannelSet(),
		ctime:        now,
		flags:        make(map[UserMode]bool),
		sasl:         NewSaslState(),
		server:       server,
		socket:       NewSocket(conn),
		replies:      make(chan string),
	}

	if _, ok := conn.(*tls.Conn); ok {
		client.flags[SecureConn] = true
	}

	client.Touch()
	go client.writeloop()
	go client.readloop()

	return client
}

//
// command goroutine
//

func (client *Client) writeloop() {
	for reply := range client.replies {
		client.socket.Write(reply)
	}
}

func (client *Client) readloop() {
	var command Command
	var err error
	var line string

	// Set the hostname for this client.
	client.hostname = AddrLookupHostname(client.socket.conn.RemoteAddr())
	client.hostmask = NewName(SHA256(client.hostname.String()))

	for err == nil {
		if line, err = client.socket.Read(); err != nil {
			command = NewQuitCommand("connection closed")

		} else if command, err = ParseCommand(line); err != nil {
			switch err {
			case ErrParseCommand:
				//TODO(dan): use the real failed numeric for this (400)
				client.Reply(RplNotice(client.server, client,
					NewText("failed to parse command")))

			case NotEnoughArgsError:
				// TODO
			}
			// so the read loop will continue
			err = nil
			continue

		} else if checkPass, ok := command.(checkPasswordCommand); ok {
			checkPass.LoadPassword(client.server)
			// Block the client thread while handling a potentially expensive
			// password bcrypt operation. Since the server is single-threaded
			// for commands, we don't want the server to perform the bcrypt,
			// blocking anyone else from sending commands until it
			// completes. This could be a form of DoS if handled naively.
			checkPass.CheckPassword()
		}

		client.processCommand(command)
	}
}

func (client *Client) processCommand(cmd Command) {
	client.server.metrics.Counter("client", "commands").Inc()

	defer func(t time.Time) {
		v := client.server.metrics.SummaryVec("client", "command_duration_seconds")
		v.WithLabelValues(cmd.Code().String()).Observe(time.Now().Sub(t).Seconds())
	}(time.Now())

	cmd.SetClient(client)

	if !client.registered {
		regCmd, ok := cmd.(RegServerCommand)
		if !ok {
			client.Quit("unexpected command")
			return
		}
		regCmd.HandleRegServer(client.server)
		return
	}

	srvCmd, ok := cmd.(ServerCommand)
	if !ok {
		client.ErrUnknownCommand(cmd.Code())
		return
	}

	switch srvCmd.(type) {
	case *PingCommand, *PongCommand:
		client.Touch()

	case *QuitCommand:
		// no-op

	default:
		client.Active()
		client.Touch()
	}

	srvCmd.HandleServer(client.server)
}

// quit timer goroutine

func (client *Client) connectionTimeout() {
	client.processCommand(NewQuitCommand("connection timeout"))
}

//
// idle timer goroutine
//

func (client *Client) connectionIdle() {
	client.server.idle <- client
}

//
// server goroutine
//

func (client *Client) Active() {
	client.atime = time.Now()
}

func (client *Client) Touch() {
	if client.quitTimer != nil {
		client.quitTimer.Stop()
	}

	if client.idleTimer == nil {
		client.idleTimer = time.AfterFunc(IDLE_TIMEOUT, client.connectionIdle)
	} else {
		client.idleTimer.Reset(IDLE_TIMEOUT)
	}
}

func (client *Client) Idle() {
	client.pingTime = time.Now()
	client.Reply(RplPing(client.server))

	if client.quitTimer == nil {
		client.quitTimer = time.AfterFunc(QUIT_TIMEOUT, client.connectionTimeout)
	} else {
		client.quitTimer.Reset(QUIT_TIMEOUT)
	}
}

func (client *Client) Register() {
	if client.registered {
		return
	}
	client.registered = true
	client.Touch()
}

func (client *Client) destroy() {
	// clean up channels

	client.channels.Range(func(channel *Channel) bool {
		channel.Quit(client)
		return true
	})

	// clean up server

	if _, ok := client.socket.conn.(*tls.Conn); ok {
		client.server.metrics.GaugeVec("server", "clients").WithLabelValues("secure").Dec()
	} else {
		client.server.metrics.GaugeVec("server", "clients").WithLabelValues("insecure").Dec()
	}

	client.server.connections.Dec()
	client.server.clients.Remove(client)

	// clean up self

	if client.idleTimer != nil {
		client.idleTimer.Stop()
	}
	if client.quitTimer != nil {
		client.quitTimer.Stop()
	}

	close(client.replies)
	client.replies = nil

	client.socket.Close()

	log.Debugf("%s: destroyed", client)
}

func (client *Client) IdleTime() time.Duration {
	return time.Since(client.atime)
}

func (client *Client) SignonTime() int64 {
	return client.ctime.Unix()
}

func (client *Client) IdleSeconds() uint64 {
	return uint64(client.IdleTime().Seconds())
}

func (client *Client) HasNick() bool {
	return client.nick != ""
}

func (client *Client) HasUsername() bool {
	return client.username != ""
}

func (client *Client) CanSpeak(target *Client) bool {
	requiresSecure := client.flags[SecureOnly] || target.flags[SecureOnly]
	isSecure := client.flags[SecureConn] && target.flags[SecureConn]
	isOperator := client.flags[Operator]

	return !requiresSecure || (requiresSecure && (isOperator || isSecure))
}

// <mode>
func (c *Client) ModeString() (str string) {
	for flag := range c.flags {
		str += flag.String()
	}

	if len(str) > 0 {
		str = "+" + str
	}
	return
}

func (c *Client) UserHost(cloacked bool) Name {
	username := "*"
	if c.HasUsername() {
		username = c.username.String()
	}
	if cloacked {
		return Name(fmt.Sprintf("%s!%s@%s", c.Nick(), username, c.hostmask))
	}
	return Name(fmt.Sprintf("%s!%s@%s", c.Nick(), username, c.hostname))
}

func (c *Client) Server() Name {
	return c.server.name
}

func (c *Client) ServerInfo() string {
	return c.server.description
}

func (c *Client) Nick() Name {
	if c.HasNick() {
		return c.nick
	}
	return Name("*")
}

func (c *Client) Id() Name {
	return c.UserHost(true)
}

func (c *Client) String() string {
	return c.Id().String()
}

func (client *Client) Friends() *ClientSet {
	friends := NewClientSet()
	friends.Add(client)
	client.channels.Range(func(channel *Channel) bool {
		channel.members.Range(func(member *Client, _ *ChannelModeSet) bool {
			friends.Add(member)
			return true
		})
		return true
	})
	return friends
}

func (client *Client) SetNickname(nickname Name) {
	if client.HasNick() {
		log.Errorf("%s nickname already set!", client)
		return
	}
	client.nick = nickname
	client.server.clients.Add(client)
}

func (client *Client) ChangeNickname(nickname Name) {
	// Make reply before changing nick to capture original source id.
	reply := RplNick(client, nickname)
	client.server.clients.Remove(client)
	client.server.whoWas.Append(client)
	client.nick = nickname
	client.server.clients.Add(client)
	client.Friends().Range(func(friend *Client) bool {
		friend.Reply(reply)
		return true
	})
}

func (client *Client) Reply(reply string) {
	if client.replies != nil {
		client.replies <- reply
	}
}

func (client *Client) Quit(message Text) {
	if client.hasQuit {
		return
	}

	client.hasQuit = true
	client.Reply(RplError("quit"))
	client.server.whoWas.Append(client)
	friends := client.Friends()
	friends.Remove(client)
	client.destroy()

	if friends.Count() > 0 {
		reply := RplQuit(client, message)
		friends.Range(func(friend *Client) bool {
			friend.Reply(reply)
			return true
		})
	}
}
