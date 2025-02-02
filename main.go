package main;

import(
  "fmt"
  "github.com/leogem2003/chatrtc/connection"
  "os"
  "log"
)

const (
  url = "ws://127.0.0.1:8000/"
)

func chat(in chan string, out chan string) {
  // really bad
	var msg string
	go func() {
		for {
			fmt.Println("msg received")
			fmt.Println(<-out)
		}
	}()
	for {
		fmt.Scan(&msg)
		in <- msg
		fmt.Println("msg sent")

	}

}

func main() {
	if len(os.Args) < 3 {
		log.Println("Missing argument")
		return
	}
	var err error
	var conn *connection.Connection
	switch os.Args[1] {
	case "answer":
		conn, err = connection.Answer(url, os.Args[2])
	case "offer":
		conn, err = connection.Offer(url, os.Args[2])
	default:
		log.Println("Error: invalid operation", os.Args[1])
		return
	}
	if err != nil {
		if conn != nil {
			conn.CloseAll()
		}
		panic(err)
	}

	go chat(conn.In, conn.Out)
	select {}
}
