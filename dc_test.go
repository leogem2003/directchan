package connection

import (
	"log"
	"testing"
	"slices"
)


func Test(t *testing.T) {
	const port = "8080"
	settings := ConnectionSettings{
		Signaling:"ws://0.0.0.0:"+port,
		STUN:[]string{"stun:stun.l.google.com:19302"},
		Key:"cd",
		BufferSize:1,
	}

	payload := []byte("test")

	go func() {
		conn1, err := FromSettings(&settings)
		defer conn1.CloseAll()
		log.Println("Created conn1")
		if err != nil {
			t.Errorf("Error while opening sender channel: %v", err)
			return
		}
		conn1.Send(payload)
		info := conn1.Recv()
		if !slices.Equal(info, payload) {
			t.Errorf("Expected %s, got %s", payload, info)
		}
		conn1.CloseAll()
	}()

	conn2, err := FromSettings(&settings)
	defer conn2.CloseAll()

	log.Println("Created conn2")
	if err != nil {
		t.Errorf("Error while opening the recv channel: %v", err)
		return
	}
	info := <- conn2.Out
	if !slices.Equal(info, payload) {
		t.Errorf("Expected %s, got %s", payload, info)
	}

	conn2.In <- payload
}

