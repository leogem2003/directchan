package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const TIMEOUT = 10 * time.Second

var Upgrader = websocket.Upgrader {
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}


type ConnPair struct {
	offerer  *websocket.Conn
	answerer *websocket.Conn
}

type ConnHandler struct {
	tmp  map[string]*CleanGuard
	tmpLock   sync.Mutex
}

type CleanGuard struct {
	pair *ConnPair
	stop chan bool
}


func (h *ConnHandler) ServeOffer(conn *websocket.Conn, key string) {
	stop := make(chan bool, 1)
	pair := ConnPair{offerer: conn, answerer: nil}
	guard := CleanGuard{pair: &pair, stop: stop}
	h.tmp[key] = &guard
	h.tmpLock.Unlock() // Instantiated guard, can do answer

	if err := conn.WriteMessage(websocket.TextMessage, []byte("OFFER")); err != nil {
		conn.Close()
		return
	}

	select {
	case <-stop:
		break
	case <-time.After(TIMEOUT):
		conn.WriteMessage(websocket.TextMessage, []byte("Fatal: timeout"))
		conn.Close()
		log.Println("timeout expired for key ", key)

		h.tmpLock.Lock()
		h.tmp[key] = nil 
		h.tmpLock.Unlock()
	}
}


func (h *ConnHandler) ServeAnswer(conn *websocket.Conn, key string) {
	pair := h.tmp[key].pair
	h.tmp[key].stop <- true
	h.tmp[key] = nil
	h.tmpLock.Unlock()

	pair.answerer = conn
	conn.WriteMessage(websocket.TextMessage, []byte("ANSWER"))
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
		c1.Close()
		c2.Close()
	}
	go relay(pair.answerer, pair.offerer)
	go relay(pair.offerer, pair.answerer)
}

func (h *ConnHandler) Connect(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	_, msg, err := conn.ReadMessage()
	log.Printf("Received request for: %s\n", msg)
	key := string(msg)

	h.tmpLock.Lock() // IMPORTANT unlock inside called functions
	guard := h.tmp[key]
	if guard == nil {
		h.ServeOffer(conn, key)
	} else {
		if guard.pair.answerer == nil {
			h.ServeAnswer(conn, key)	
		} else {
			h.tmpLock.Unlock()	
			conn.WriteMessage(websocket.TextMessage, []byte("KO: slot already allocated"))
			conn.Close()
		}
	}
}

func main() {
	handler := new(ConnHandler)
	handler.tmp = make(map[string]*CleanGuard)
	http.HandleFunc("/", handler.Connect)
	port := os.Args[1]
	log.Println("Serving on port ", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
