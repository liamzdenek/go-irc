package main

import (
	"log"

	"github.com/liamzdenek/go-irc/irc"
	"github.com/liamzdenek/go-irc/irc/irce"
)

func main() {
	i := irc.NewIRC("hypeirc:6667")
	ch := irce.NewChannelHandler(i);
	
	log.Printf("Entering main loop\n");
	for e := range i.Rx {
		// prints nice messages to stdout... great for debugging/logging
		// this line can be put after the PingHandler (given that you're 
		// continuing when it returns true) and it won't print Pings
		irce.LogHandler(e);

		// handles PING/PONG automatically. You generally want this
		if i.PingHandler(e) {
			// prevents other handlers from wasting their time
			// when this ping has already been handled. we can just abort
			// early
			continue;
		}

		ch.Handle(e);

		// Extras TODO:
		// * ChannelHandler, manages the channels and the users in them.
		//   * will also auto-rejoin channels on disconnect
		// * PermissionHandler, interface. comes with very simple builtin
		//   * UserCan(user User, permKey string) bool
		// * CommandParser, function, just parses a privmsg into argument list

		// Custom stuffs
		switch l := e.(type) {
		case *irc.EConnect:
			i.Tx <- &irc.Line{
				Command: "NICK",
				Arguments: []string{"LiamTest"},
			}
			i.Tx <- &irc.Line{
				Command: "USER",
				Arguments: []string{"LiamTest", "8", "*"},
				Suffix: "Liam Test",
			}
		case *irc.Line:
			switch l.Command {
			case "001":
				i.Tx <- &irc.Line{
					Command: "JOIN",
					Arguments: []string{"#rust-nuts"},
				}
			}
		}
	}
}
