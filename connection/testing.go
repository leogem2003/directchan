package connection

import (
	"testing"
)

func TestConnection(t *testing.T) {
	conn := new(Connection)
	conn.MakeWSConnection("ws://127.0.0.1:8000/offer", "key")

	// try to create a connection with the same key
	conn2 := new(Connection)
	_, err := conn2.MakeWSConnection("ws://127.0.0.1:8000/answer", "key")
	if err != nil {
		t.Fatalf("Failed offer/answer websocket connection: %v\n", err)
	}
}

func TestDataChannel(t *testing.T) {
	conn1, err := Offer("ws:127.0.0.1:8000/", "key")
	if err != nil {
		t.Fatalf("Failed to create offer: %v\n", err)
	}
	conn2, err := Answer("ws:127.0.0.1:8000/", "key")
	if err != nil {
		t.Fatalf("Failed to create answer: %v\n", err)
	}
	done := make(chan bool, 2)
	go func() {
		var recv string
		conn1.In <- []byte("heyo")
		if recv != "resp" {
			t.Fatalf("Invalid string received: %s\n", recv)
		}
		done <- true
	}()

	go func() {
		var recv string
		recv = string(<-conn2.Out)
		if recv != "heyo" {
			t.Fatalf("Invalid string received: %s\n", recv)
		}
		conn2.In <- []byte("resp")
		done <- true
	}()

	<-done
	<-done
}
