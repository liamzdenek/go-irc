package irc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const TRIM = " \r\n"

type IRC struct {
	Tx     chan Event // transmit
	Rx     chan Event // receive
	server string
}

type Event interface {
	Send(conn net.Conn) error
}

type EConnect struct{}

func (ec *EConnect) Send(conn net.Conn) error {
	panic("The EConnect type cannot be sent")
	return nil
}

type EDisconnect struct{}

func (ec *EDisconnect) Send(conn net.Conn) error {
	panic("The EDisconnect type cannot be sent")
	return nil
}

type Line struct {
	Prefix    string
	Command   string
	Arguments []string
	Suffix    string
}

func (el *Line) Send(conn net.Conn) error {
	line := el.Raw()
	log.Printf("Sending Line: %s", line)
	conn.Write([]byte(line))
	return nil
}

func (el *Line) Raw() (s string) {
	if len(el.Prefix) > 0 {
		s = ":" + el.Prefix + " "
	}

	s = s + el.Command + " "

	if len(el.Arguments) > 0 {
		s = s + strings.Join(el.Arguments, " ") + " "
	}

	if len(el.Suffix) > 0 {
		s = s + ":" + el.Suffix
	}

	s = s + "\r\n"
	return
}

type LineBuilder struct {
	prefix    string
	command   string
	arguments []string
	suffix    string
}

type ENoSpaces struct{ field string }

func (e *ENoSpaces) Error() string {
	return fmt.Sprintf("MUST not contain any spaces", e.field)
}

type ENoNewlines struct{ field string }

func (e *ENoNewlines) Error() string {
	return fmt.Sprintf("MUST not contain any \\n or \\r - %s", e.field)
}

type EMissingCommand struct{}

func (e *EMissingCommand) Error() string {
	return "A line MUST contain a non-empty .Command"
}

func NewLineBuilder() *LineBuilder {
	return &LineBuilder{
		prefix:    "",
		command:   "",
		arguments: []string{},
		suffix:    "",
	}
}

func (lb *LineBuilder) Prefix(p string) *LineBuilder {
	lb.prefix = p
	return lb
}

func (lb *LineBuilder) Command(c string) *LineBuilder {
	lb.command = c
	return lb
}

func (lb *LineBuilder) PushArg(a string) *LineBuilder {
	lb.arguments = append(lb.arguments, a)
	return lb
}

func (lb *LineBuilder) ArgsFromString(a string) *LineBuilder {
	lb.arguments = strings.Split(a, " ")
	return lb
}

func (lb *LineBuilder) Suffix(s string) *LineBuilder {
	lb.suffix = s
	return lb
}

func (lb *LineBuilder) Sanitize() *LineBuilder {
	// prefix must be one word
	lb.prefix = strings.Replace(lb.prefix, " ", "", -1)

	//command must be one word
	lb.command = strings.Replace(lb.command, " ", "", -1)

	// no \r\n anywhere
	lb.prefix = strings.Replace(lb.command, "\r", "", -1)
	lb.prefix = strings.Replace(lb.command, "\n", "", -1)
	lb.command = strings.Replace(lb.command, "\r", "", -1)
	lb.command = strings.Replace(lb.command, "\n", "", -1)
	for i, arg := range lb.arguments {
		lb.arguments[i] = strings.Replace(arg, "\r", "", -1)
		lb.arguments[i] = strings.Replace(arg, "\n", "", -1)
	}
	lb.suffix = strings.Replace(lb.suffix, "\r", "", -1)
	lb.suffix = strings.Replace(lb.suffix, "\n", "", -1)

	return lb
}

func (lb *LineBuilder) Consume() (*Line, error) {
	if strings.Contains(lb.prefix, " ") {
		return nil, &ENoSpaces{field: "prefix"}
	}
	if strings.ContainsAny(lb.prefix, "\r\n") {
		return nil, &ENoNewlines{field: "prefix"}
	}
	for i, arg := range lb.arguments {
		if strings.Contains(arg, " ") {
			return nil, &ENoSpaces{field: fmt.Sprintf("arg[%d]", i)}
		}
		if strings.ContainsAny(arg, "\r\n") {
			return nil, &ENoNewlines{field: fmt.Sprintf("arg[%d]", i)}
		}
	}
	if len(lb.command) == 0 {
		return nil, &EMissingCommand{}
	}
	if strings.Contains(lb.command, " ") {
		return nil, &ENoNewlines{field: "command"}
	}
	if strings.ContainsAny(lb.command, "\r\n") {
		return nil, &ENoNewlines{field: "command"}
	}
	return &Line{
		Prefix:    lb.prefix,
		Command:   lb.command,
		Arguments: lb.arguments,
		Suffix:    lb.suffix,
	}, nil
}

func NewLineFromRaw(line string) (*Line, error) {
	line = strings.Trim(line, TRIM)

	result := &Line{
		Prefix:    "",
		Command:   "",
		Arguments: []string{},
		Suffix:    "",
	}

	prefixEnd := -1
	trailingStart := len(line)

	if trailingStart == 0 {
		return nil, errors.New("Line is 0 characters long. This is too short")
	}

	//determine the prefix if one is present
	if string(line[0]) == ":" {
		if i := strings.Index(line, " "); i != -1 {
			prefixEnd = i
			result.Prefix = line[1:i]
		}
		// else { no prefix is present. no problemo }
	}

	//determine if a suffix is present
	if i := strings.Index(line, " :"); i != -1 {
		trailingStart = i
		result.Suffix = line[i+2:]
	}
	// else { no suffix is present. no problemo }

	params_str := line[prefixEnd+1 : trailingStart]

	params := strings.Split(params_str, " ")

	if len(params) == 0 {
		return nil, errors.New("There is no command")
	}

	result.Command = params[0]

	if len(params) > 1 {
		result.Arguments = params[1:]
	}

	return result, nil
}

func NewIRC(server string) *IRC {
	i := &IRC{
		Tx:     nil,
		Rx:     make(chan Event),
		server: server,
	}
	go i.Run()
	return i
}

func (i *IRC) PingHandler(e Event) bool {
	handled := false

	switch l := e.(type) {
	case *Line:
		switch l.Command {
		case "PING":
			i.Tx <- &Line{
				Command:   "PONG",
				Arguments: l.Arguments,
			}
		}
	}

	return handled
}

func (i *IRC) Run() {
	var sock net.Conn = nil
	for {
		if sock == nil {
			log.Printf("Dialing...")
			c, err := i.connect()
			if err != nil {
				log.Printf("Could not connect. Retrying in 10s: %s\n", err)
				time.Sleep(10 * time.Second)
				continue
			}
			log.Printf("Dialed")
			sock = c
			i.Tx = make(chan Event)
			i.Rx <- &EConnect{}
		}
		go i.writeRoutine(sock)
		bufio := bufio.NewReader(sock)
		for {
			sock.SetReadDeadline(time.Now().Add(time.Second * 60))
			line, err := bufio.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					log.Printf("Got EOF. read thread restarting")
					close(i.Tx)
					sock = nil
					break
				}
				log.Printf("Reading error: %s\n", err)
				continue
			}
			iline, err := NewLineFromRaw(line)
			if err != nil {
				log.Printf("Line parsing error: %s\n", err)
				continue
			}
			i.Rx <- iline
		}
		i.Rx <- &EDisconnect{}
	}
}

func (i *IRC) writeRoutine(conn net.Conn) {
	for w := range i.Tx {
		if e := w.Send(conn); e != nil {
			if e == io.EOF {
				log.Printf("Got EOF in write thread. write thread exiting")
				return
			}
			log.Printf("Sending error: %s\n", e)
			continue
		}
	}
	log.Printf("Write thread terminating")
}

// TODO: use a net.Dialer -- should provide us with proxy support
func (i *IRC) connect() (net.Conn, error) {
	return net.Dial("tcp", i.server)
}
