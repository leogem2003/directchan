package main
import (
	"testing"
	"net/http"
	"log"
	"os"
	ws "github.com/gorilla/websocket"
)

func startServer() {
	handler := new(ConnHandler)
	handler.tmp = make(map[string]*CleanGuard)
	http.HandleFunc("/", handler.Connect)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func TestConnect(t *testing.T) {
	key := "ab"
	conn1, _, err := ws.DefaultDialer.Dial("ws://localhost:8080/", nil)
	if err != nil {
		t.Errorf("Error while opening connection 1: %v", err)
		return
	}

	conn2, _, err := ws.DefaultDialer.Dial("ws://localhost:8080/", nil)
	if err != nil {
		t.Errorf("Error while opening connection 2: %v", err)
		return
	}

	if err = conn1.WriteMessage(ws.TextMessage, []byte(key)); err != nil {
		t.Errorf("Error while allocating key: %v", err)
		return
	}

	_, answer1, err := conn1.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving OK1 %v", err)
		return
	} else if string(answer1) != "OFFER" {
		t.Errorf("Expected OFFER, got %s", string(answer1))
	}

	if err = conn2.WriteMessage(ws.TextMessage, []byte(key)); err != nil {
		t.Errorf("Error while answering for key: %v", err)
		return
	}

	_, answer2, err := conn2.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving OK2 %v", err)
		return
	} else if string(answer2) != "ANSWER" {
		t.Errorf("Expected ANSWER, got %s", string(answer2))
	}


	_, answer1, err = conn1.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving Ready1 %v", err)
		return
	} else if string(answer1) != "Ready" {
		t.Errorf("Expected Ready, got %s", string(answer1))
	}

	_, answer2, err = conn2.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving Ready2 %v", err)
		return
	} else if string(answer2) != "Ready" {
		t.Errorf("expected Ready, got %s", string(answer2))
	}

	conn1.WriteMessage(ws.TextMessage, []byte("hoi"))
	_, answer2, err = conn2.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving msg from 1 %v", err)
		return
	} else if string(answer2) != "hoi" {
		t.Errorf("expected hoi, got %s", string(answer2))
	}

	
	conn2.WriteMessage(ws.TextMessage, []byte("hey"))
	_, answer1, err = conn1.ReadMessage()
	if err != nil {
		t.Errorf("Error while receiving Ready2 %v", err)
		return
	} else if string(answer1) != "hey" {
		t.Errorf("expected hey, got %s", string(answer1))
	}
	
	conn1.Close()
	if _, _, err := conn2.ReadMessage(); err==nil {
		t.Errorf("conn2 should be closed")
		return
	}
}

func TestMain(m *testing.M) {
	go startServer()
	code := m.Run()
	os.Exit(code)
}
