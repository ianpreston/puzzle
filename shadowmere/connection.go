package shadowmere

import (
	"../kenny"
	"bufio"
	"fmt"
	"net"
	"strings"
)

type handler func(string, []string)

type Connection struct {
	mere *Services

	conn   net.Conn
	reader *bufio.Reader

	handlers map[string]handler

	name string
	addr string
	pass string
}

func NewConnection(mere *Services, name, addr, pass string) (*Connection, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)

	srv := &Connection{
		mere: mere,

		conn:   conn,
		reader: reader,

		name: name,
		addr: addr,
		pass: pass,
	}
	srv.handlers = map[string]handler{
		"PING":    srv.handlePing,
		"PRIVMSG": srv.handlePrivmsg,
		"QUIT":    srv.handleQuit,
		"NICK":    srv.handleNick,
	}

	return srv, nil
}

func (srv *Connection) Start() {
	srv.authenticateUnreal()
	srv.initializeServices()
	srv.listenLoop()
}

func (srv *Connection) authenticateUnreal() {
	// Implements UnrealIRCd-compatible aurhentication
	srv.write(fmt.Sprintf("PASS :%s\r\n", srv.pass))
	srv.write(fmt.Sprintf("PROTOCTL %s\r\n", "SJ3 NICKv2 NOQUIT"))
	srv.write(fmt.Sprintf("SERVER %s 1 :%s\r\n", srv.name, "Services"))
}

func (srv *Connection) initializeServices() {
	ns := srv.mere.nickserv
	cs := srv.mere.chanserv

	srv.nick(ns.Nick, ns.Nick, srv.name, "Services")
	srv.nick(cs.Nick, cs.Nick, srv.name, "Services")
}

func (srv *Connection) listenLoop() {
	for {
		line, err := srv.read()
		if err != nil {
			kenny.CriticalErr(err)
			return
		}

		srv.handleLine(line)
	}
}

func (srv *Connection) handleLine(line string) {
	command, origin, args, err := srv.parseMessage(line)
	if err != nil {
		kenny.Warn("handleLine(): " + err.Error())
		return
	}

	h := srv.handlers[command]
	if h != nil {
		h(origin, args)
	}
}

func (srv *Connection) handlePing(origin string, args []string) {
	if len(args) == 0 {
		kenny.Warn("Malformed PING")
		return
	}

	srv.pong(args[0])
}

func (srv *Connection) handlePrivmsg(origin string, args []string) {
	if len(args) < 2 {
		kenny.Warn("Malformed PRIVMSG")
		return
	}

	to := args[0]
	msg := args[1]
	if strings.ToLower(srv.mere.nickserv.Nick) == strings.ToLower(to) {
		srv.mere.nickserv.OnPrivmsg(origin, msg)
	}
	if strings.ToLower(srv.mere.chanserv.Nick) == strings.ToLower(to) {
		srv.mere.chanserv.OnPrivmsg(origin, msg)
	}
}

func (srv *Connection) handleQuit(origin string, args []string) {
	var msg string
	if len(args) > 0 {
		msg = args[0]
	}

	srv.mere.nickserv.OnQuit(origin, msg)
}

func (srv *Connection) handleNick(origin string, args []string) {
	if origin == "" {
		// Server introducing a new user
		// nick hopcount timestamp	username hostname server servicestamp +usermodes virtualhost :realname
		srv.handleNewNick(args)
	} else {
		// User changing their nick
		// :old nick new timestamp
		srv.handleNickChange(origin, args)
	}
}

func (srv *Connection) handleNewNick(args []string) {
	if len(args) < 1 {
		kenny.Warn("Malformed NICKv2")
		return
	}

	newNick := args[0]
	srv.mere.nickserv.OnNewNick(newNick)
}

func (srv *Connection) handleNickChange(origin string, args []string) {
	if len(args) < 1 {
		kenny.Warn("Malformed NICK")
		return
	}

	oldNick := origin
	newNick := args[0]
	srv.mere.nickserv.OnNickChange(oldNick, newNick)
}

func (srv *Connection) read() (string, error) {
	s, err := srv.reader.ReadString('\n')

	return s, err
}

func (srv *Connection) write(s string) {
	fmt.Fprint(srv.conn, s)
}
