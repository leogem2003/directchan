package main

import (
	"fmt"
	"log"
	"os"

	"github.com/leogem2003/directchan"
)

func chat(c *connection.Connection) {
	// output daemon
	go func() {
		for {
			fmt.Println(string(<-c.Out))
		}
	}()

	// state daemon
	go func() {
		for {
			fmt.Println("State changed!");
			fmt.Println((<-c.State).String())
		}
	}()

	var msg string
	for {
		fmt.Scan(&msg)
		c.In <- []byte(msg)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Missing argument. Usage: example <ws address> key")
		return
	}
	var err error
	var conn *connection.Connection
	settings := new(connection.ConnectionSettings)
	settings.Key = os.Args[2]
	settings.STUN = []string{"stun:stun.l.google.com:19302"}
	settings.Signaling = os.Args[1]
	settings.BufferSize = 1

	conn, err = connection.FromSettings(settings)
	if err != nil {
		log.Fatalln(err)
	}

	chat(conn)
}
