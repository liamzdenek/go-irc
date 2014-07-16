package irce

import (
	".."
	"log"
)

func LogHandler(e irc.Event) {
	log.Printf("GOT EVENT: %s\n", e);
	switch l := e.(type) {
	case *irc.EConnect:
		log.Printf("Connected\n")
	case *irc.EDisconnect:
		log.Printf("Disconnected\n")
	case *irc.Line:
		log.Printf(
			"Got Line: \n"+
				"\tPrefix:    %s\n"+
				"\tCommand:   %s\n"+
				"\tArguments: %s\n"+
				"\tSuffix:    %s\n\t",
			l.Prefix,
			l.Command,
			l.Arguments,
			l.Suffix,
		)
	}
}
