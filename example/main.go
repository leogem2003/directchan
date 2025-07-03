package main

import (
	"fmt"
	"github.com/leogem2003/directchan"
	"log"
	"os"
)

func chat(in chan []byte, out chan []byte) {
	// really bad
	var msg string
	go func() {
		for {
			fmt.Println(string(<-out))
		}
	}()
	for {
		fmt.Scan(&msg)
		in <- []byte(msg)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Missing argument")
		return
	}
	var err error
	var conn *connection.Connection
	settings := new(connection.ConnectionSettings)
	settings.Key = os.Args[3]
	settings.STUN = []string{"stun:stun.l.google.com:19302"}
	settings.Signaling = os.Args[1]
	settings.BufferSize = 1
	settings.Operation = os.Args[2]
	switch os.Args[2] {
	case "answer":
		conn, err = connection.Answer(settings)
	case "offer":
		conn, err = connection.Offer(settings)
	default:
		log.Println("Error: invalid operation", os.Args[1])
		return
	}
	if err != nil {
		if conn != nil {
			conn.CloseAll()
		}
		log.Fatalln(err)
	}

	chat(conn.In, conn.Out)
}
