package irce

import (
	"log"
	"strings"

	"github.com/liamzdenek/go-irc/irc"
)

type ChannelHandler struct {
	irc       *irc.IRC
	channels  map[string][]string
	nameSwap  []string
	joinQueue *[]string
}

func NewChannelHandler(i *irc.IRC) *ChannelHandler {
	return &ChannelHandler{
		irc:       i,
		channels:  make(map[string][]string),
		nameSwap:  []string{},
		joinQueue: &[]string{},
	}
}

func (ch *ChannelHandler) Part(c string) {
	if ch.channels[c] == nil {
		// already in the channel, do nothing
		return
	}
	if ch.joinQueue == nil {
		go func() {
			ch.irc.Tx <- &irc.Line{
				Command:   "PART",
				Arguments: []string{c},
			}
		}()
	} else {
		ch.popJoinQueue(c)
	}
}

func (ch *ChannelHandler) Join(c string) {
	if ch.channels[c] != nil {
		// already in the channel, do nothing
		return
	}

	if ch.joinQueue == nil {
		go func() {
			ch.irc.Tx <- &irc.Line{
				Command:   "JOIN",
				Arguments: []string{c},
			}
		}()
	} else {
		ch.pushJoinQueue(c)
	}
}

func (ch *ChannelHandler) pushJoinQueue(c string) {
	a := append(*ch.joinQueue, c)
	ch.joinQueue = &a
}

func (ch *ChannelHandler) popJoinQueue(c string) {
	for i, v := range *ch.joinQueue {
		if v == c {
			k := *ch.joinQueue;
			k = append(k[:i], k[i+1:]...);
			ch.joinQueue = &k;
			break;
		}
	}
}

func (ch *ChannelHandler) Handle(e irc.Event) {
	switch l := e.(type) {
	case *irc.EDisconnect:
		ch.joinQueue = &[]string{}
		for c, _ := range ch.channels {
			delete(ch.channels, c)
			ch.pushJoinQueue(c)
		}
	case *irc.Line:
		switch l.Command {
		case "001":
			if ch.joinQueue != nil {
				q := *ch.joinQueue
				ch.joinQueue = nil
				for _, channel := range q {
					ch.Join(channel)
				}
			}
		case "JOIN":
			channel := l.Suffix
			if len(channel) == 0 || ch.channels[channel] == nil {
				// this is probably just the server telling us that we have joined a
				//log.Printf("Malformed JOIN command received")
				break
			}
			nick := strings.SplitN(l.Prefix, "!", 2)[0]

			log.Printf("User '%s' joined channel '%s'", nick, channel)
			ch.channels[channel] = append(ch.channels[channel], nick)
		case "PART":
			if len(l.Arguments) == 0 {
				log.Printf("Got a PART but the arguments did not contain a channel")
				break
			}
			if strings.Index(l.Prefix, "!") == -1 {
				log.Printf("Got a PART but the user did not contain an nick and an ident")
				break
			}
			channel := l.Arguments[0]
			if ch.channels[channel] == nil {
				log.Printf("Got a PART for a channel that we didn't know about (%s)", channel)
			}

			nick := strings.SplitN(l.Prefix, "!", 2)[0]

			found := false
			for i, c := range ch.channels[channel] {
				if c == nick {
					found = true
					ch.channels[channel] = append(ch.channels[channel][:i], ch.channels[channel][i+1:]...)
					log.Printf("GOT USER IN LIST: %s", c)
				}
			}
			if !found {
				log.Printf("Got a PART for a user that is not in the channel: %s %s", channel, nick)
			} else {
				log.Printf("New user list for %s: %s", channel, ch.channels[channel])
			}
		case "353":
			if len(l.Arguments) == 0 || len(l.Suffix) == 0 {
				log.Printf("Not enough arguments in 353 to build an initial user list")
				break
			}
			channel := l.Arguments[len(l.Arguments)-1]
			if ch.channels[channel] == nil {
				ch.channels[channel] = []string{}
			}
			names := strings.Split(l.Suffix, " ")
			ch.nameSwap = append(ch.nameSwap, names...)
		case "366":

			var names []string = nil
			names, ch.nameSwap = ch.nameSwap, []string{}
			if len(l.Arguments) == 0 {
				log.Printf("Not enough arguments in 366 to determine which channel we finished joining")
				break
			}
			channel := l.Arguments[len(l.Arguments)-1]
			ch.channels[channel] = names
			log.Printf("Joined channel '%s' with %d users (%s)", channel, len(names), names)
		}
	}
}
