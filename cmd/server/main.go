package main

import (
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type connPair struct {
	offerer  *websocket.Conn
	answerer *websocket.Conn
}

type connHandler struct {
	pool map[string]*connPair
	mu   sync.Mutex
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
	if h.pool[key] != nil {
		h.mu.Unlock()
		conn.WriteMessage(websocket.TextMessage, []byte("KO: key already in use"))
		conn.Close()
		return
	}
	pair := connPair{offerer: conn}
	h.pool[key] = &pair
	h.mu.Unlock()

	conn.WriteMessage(websocket.TextMessage, []byte("OK"))
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
	pair := h.pool[key]
	h.mu.Unlock()

	if pair == nil || pair.answerer != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("KO"))
		conn.Close()
		return
	}

	pair.answerer = conn
	log.Println("added to group")

	conn.WriteMessage(websocket.TextMessage, []byte("OK"))
	pair.offerer.WriteMessage(websocket.TextMessage, []byte("Ready"))
	conn.WriteMessage(websocket.TextMessage, []byte("Ready"))
	// Starts signaling exchange
	relay := func(c1 *websocket.Conn, c2 *websocket.Conn, msg string) {
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
	go relay(pair.answerer, pair.offerer, "a->o")
	go relay(pair.offerer, pair.answerer, "o->a")
}

func main() {
	handler := new(connHandler)
	handler.pool = make(map[string]*connPair)
	http.HandleFunc("/offer", handler.ServeOffer)
	http.HandleFunc("/answer", handler.ServeAnswer)
	log.Println("Serving on port 8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
