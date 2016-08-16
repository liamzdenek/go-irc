package main

import (
	"fmt"
	"log"
	"time"
	"strings"

	"github.com/liamzdenek/go-irc/irc"
	"github.com/liamzdenek/go-irc/irc/irce"
)

func main() {
	i := irc.NewIRC(Conf.Server)
	ch := irce.NewChannelHandler(i)

	name := Conf.Name

	for channel, channel_data := range Conf.Channels {
		ch.Join(channel)
		for _, feed_str := range channel_data.Feeds {
			go func(f, c string) {
				feed := NewRSSFeed(f, NewRamCache())
				for item := range feed.Rx {
					time.Sleep(time.Second * time.Duration(len(Conf.Channels)))
					line, err := irc.NewLineBuilder().
						Command("PRIVMSG").
						ArgsFromString(c).
						Suffix(fmt.Sprintf("%s - %s", item.Title, item.Link)).
						Sanitize().
						Consume()
					if err != nil {
						fmt.Println("Error generating RSS announcement: %s", err)
					} else {
						i.Tx <- line
					}
				}
			}(feed_str, channel)
		}
	}

	log.Printf("Entering main loop\n")
	//for e := range i.Rx {
	for {
		select {
		case e := <-i.Rx:
			// prints nice messages to stdout... great for debugging/logging
			// this line can be put after the PingHandler (given that you're
			// continuing when it returns true) and it won't print Pings
			irce.LogHandler(e)

			// handles PING/PONG automatically. You generally want this
			if i.PingHandler(e) {
				// prevents other handlers from wasting their time
				// when this ping has already been handled. we can just abort
				// early
				continue
			}

			ch.Handle(e)

			// Custom stuffs
			switch l := e.(type) {
			case *irc.EConnect:
				i.Tx <- &irc.Line{
					Command:   "NICK",
					Arguments: []string{name},
				}
				i.Tx <- &irc.Line{
					Command:   "USER",
					Arguments: []string{name, "8", "*"},
					Suffix:    Conf.Realname,
				}
			case *irc.Line:
				switch l.Command {
				case "JOIN":
					c := l.Suffix;
					if Conf.Channels[c] != nil {
						log.Printf("GOT A JOIN IN: %s\n", c);
						p := strings.Split(l.Prefix, "@");
						if len(p) > 1 {
							ident := p[len(p)-1];
							name_parts := strings.Split(p[0], "!");
							name := name_parts[0];
							for _, person := range Conf.Channels[c].Ops {
								log.Printf("NAME: %s, IDENT: %s PERSON: %s\n", name, ident, person);
								if(person == ident) {
									i.Tx <- &irc.Line{
										Command:   "MODE",
										Arguments: []string{c, "+o", name},
									}
								}
							}
						}
					}
				case "001":
					/*i.Tx <-&irc.Line{
						Command: "PRIVMSG",
						Arguments: []string{"#rust-nuts"},
						Suffix: "LOL THING",
					};*/
				}
			}
		}
	}
}
