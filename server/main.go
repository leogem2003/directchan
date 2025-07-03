package main

import (
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const TIMEOUT = 10 * time.Second

var upgrader = websocket.Upgrader {
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type connPair struct {
	offerer  *websocket.Conn
	answerer *websocket.Conn
}

type connHandler struct {
	pool map[string]*connPair
	tmp  map[string]*cleanGuard
	mu   sync.Mutex
}

type cleanGuard struct {
	key  string
	conn *websocket.Conn
	stop chan bool
}

func (h *connHandler) FreeKey(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pool[key] = nil
}

func (h *connHandler) ServeOffer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// 1. Create the chat
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Println(err)
		return
	}

	key := string(msg)
	log.Printf("Received key: %s\n", key)

	h.mu.Lock()
	if h.pool[key] != nil || h.tmp[key] != nil {
		h.mu.Unlock()
		conn.WriteMessage(websocket.TextMessage, []byte("KO: key already in use"))
		conn.Close()
		return
	}
	stop := make(chan bool, 1)
	guard := cleanGuard{conn: conn, stop: stop}
	h.tmp[key] = &guard
	h.mu.Unlock()

	conn.WriteMessage(websocket.TextMessage, []byte("OK"))

	select {
	case <-stop:
		break
	case <-time.After(TIMEOUT):
		guard.conn.WriteMessage(websocket.TextMessage, []byte("Fatal: timeout"))
		guard.conn.Close()
		log.Println("timeout expired for key ", key)
	}
	h.mu.Lock()
	h.tmp[key] = nil
	h.mu.Unlock()
}


func (h *connHandler) ServeAnswer(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	_, msg, err := conn.ReadMessage()
	log.Printf("Received request for: %s\n", msg)
	key := string(msg)

	h.mu.Lock()
	guard := h.tmp[key]
	pair := h.pool[key]
	if guard == nil || pair != nil {
		h.mu.Unlock()
		conn.WriteMessage(websocket.TextMessage, []byte("KO"))
		conn.Close()
		return
	}
	guard.stop <- true
	pair = &connPair{offerer: guard.conn, answerer: conn}
	h.pool[key] = pair
	log.Println("added to group")
	h.mu.Unlock()

	conn.WriteMessage(websocket.TextMessage, []byte("OK"))
	pair.offerer.WriteMessage(websocket.TextMessage, []byte("Ready"))
	conn.WriteMessage(websocket.TextMessage, []byte("Ready"))
	// Starts signaling exchange
	relay := func(c1 *websocket.Conn, c2 *websocket.Conn) {
		for {
			t, reader, err := c1.NextReader()
			if err != nil {
				break
			}
			p, err := io.ReadAll(reader)
			if err != nil {
				break
			}
			writer, err := c2.NextWriter(t)
			if err != nil {
				break
			}
			writer.Write(p)
			writer.Close()
		}
		log.Println("closing ", key)
		h.FreeKey(key)
		c1.Close()
		c2.Close()
	}
	go relay(pair.answerer, pair.offerer)
	go relay(pair.offerer, pair.answerer)
}

func main() {
	handler := new(connHandler)
	handler.pool = make(map[string]*connPair)
	handler.tmp = make(map[string]*cleanGuard)
	http.HandleFunc("/offer", handler.ServeOffer)
	http.HandleFunc("/answer", handler.ServeAnswer)
	port := os.Args[1]
	log.Println("Serving on port ", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
