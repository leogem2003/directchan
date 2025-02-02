package connection

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"log"
	"os"
)

const (
	url = "wss://127.0.0.1:8000/"
)

type Connection struct {
	sock *websocket.Conn
	peer *webrtc.PeerConnection
	Out  chan string
	In   chan string
}

func (c *Connection) MakeWSConnection(url string, key string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return conn, err
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte(key))
	if err != nil {
		return conn, err
	}

	// OK
	_, resp, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return conn, err
	}
	if string(resp) != "OK" {
		conn.Close()
		return conn, errors.New("Bad response: " + string(resp))
	}

	// Ready
	_, resp, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		return conn, err
	}
	if string(resp) != "Ready" {
		conn.Close()
		return conn, errors.New("Bad response: " + string(resp))
	}
	c.sock = conn
	return conn, nil
}

func (c *Connection) SignalCandidate(candidate *webrtc.ICECandidate) error {
	candidateJSON := []byte(candidate.ToJSON().Candidate)
	return c.sock.WriteJSON(map[string][]byte{
		"type": []byte("candidate"),
		"ice":  candidateJSON,
	})
}

func (c *Connection) MakePeerConnection() error {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peer_conn, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}
	c.peer = peer_conn

	peer_conn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			// Send the ICE candidate to the signaling server
			log.Println("Sending ICE candidate")
			if err := c.SignalCandidate(candidate); err != nil {
				log.Printf("Failed to send ICE candidate: %v", err)
			}
		}
	})

	peer_conn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("Connection state changed: ", state.String())
		if state == webrtc.PeerConnectionStateClosed {
			log.Println("exiting...")
			os.Exit(0)
		} else if state == webrtc.PeerConnectionStateFailed {
			log.Println("exiting on failure...")
			os.Exit(0)
		}
	})

	return nil
}

func (c *Connection) ConsumeSignaling() error {
	var message map[string][]byte
	var sdp webrtc.SessionDescription
	for {
		if err := c.sock.ReadJSON(&message); err != nil {
			return err
		}
		switch string(message["type"]) {
		case "offer":
			// 1. Set the offer as RemoteDescription
			if err := json.Unmarshal(message["sdp"], &sdp); err != nil {
				return err
			}

			log.Println("Received offer")
			if err := c.peer.SetRemoteDescription(sdp); err != nil {
				return err
			}

			// 2. Create an SDP answer
			answer, err := c.peer.CreateAnswer(nil)
			if err != nil {
				return err
			}

			// 3. Set the local description with the answer
			if err := c.peer.SetLocalDescription(answer); err != nil {
				return err
			}

			// 4. Send the SDP answer to the signaling server
			answerJSON, err := json.Marshal(answer)
			if err != nil {
				return err
			}

			err = c.sock.WriteJSON(map[string][]byte{
				"type": []byte("answer"),
				"sdp":  answerJSON,
			})

			log.Println("sent answer")
			if err != nil {
				return err
			}
		case "answer":
			// 1. Set the offer as RemoteDescription
			if err := json.Unmarshal(message["sdp"], &sdp); err != nil {
				return err
			}

			log.Println("Received answer")
			if err := c.peer.SetRemoteDescription(sdp); err != nil {
				log.Fatalf("Invalid remote offer %v\n", err)
				return err
			}
		case "candidate":
			log.Println("Received ICE candidate")
			if err := c.peer.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(message["ice"])},
			); err != nil {
				return err
			}
		default:
			log.Println("unknown signal:", message["type"])
		}
	}
}

func (c *Connection) CreateDataChannel() (*webrtc.DataChannel, error) {
	return c.peer.CreateDataChannel("data", nil)
}

func (c *Connection) CloseAll() error {
	if c.sock != nil {
		if err := c.sock.Close(); err != nil {
			return err
		}
	}
	if c.peer != nil {
		if err := c.peer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Connection) AttachFunctionality(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("channel opened!")
		dc.SendText("Greetings from offerer")
		var msg string
		for {
			log.Println("Waiting user input")
			msg = <-c.In
			dc.SendText(msg)
			log.Println("Sent ", msg)
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println("Received message: ", string(msg.Data))
		c.Out <- string(msg.Data)
	})
}

func (c *Connection) CreateBuffers(size uint) error {
	if size == 0 {
		return errors.New("Invalid size for channel: 0")
	}
	in := make(chan string, size)
	out := make(chan string, size)
	c.In = in
	c.Out = out
	return nil
}
