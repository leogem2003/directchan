package connection

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"fmt"
	"sync"
)


// Settings for a Connection object
type ConnectionSettings struct {
	Signaling string // address of the signaling server ("ws://<ip>:<port>")
	STUN []string
	TURN string
	Key string // Channel's identifier
	BufferSize uint // Size in bytes of the output/input buffers
	Operation int // (0 = "offer" | 1 = "answer")
}

type Connection struct {
	// Signaling connection (websocket)
	sock *websocket.Conn
	// Peer connection (webrtc)
	peer *webrtc.PeerConnection

	// IO buffers
	// connection output (received from remote)
	Out chan []byte
	// connection input (send to remote)
	In  chan []byte

	// State channel
	State chan webrtc.PeerConnectionState 

	// settings
	Settings *ConnectionSettings

	// true iff CloseAll has been called
	IsClosed bool

	mu sync.Mutex	
}

// Instantiates a new connection from its settings.
// Basically, decide if it's an offer (settings.Operation==0)
// or an answer (settings.Operation==1).
// Also checks for the correctness of the settings field
func FromSettings(settings *ConnectionSettings) (*Connection, error) {
	if settings.BufferSize == 0 {
		return nil, errors.New("Buffer size must be greater than 0")
	}

	if settings.Operation == 0 {
		return Offer(settings)
	} else if settings.Operation == 1 {
		return Answer(settings)
	}

	return nil, errors.New(fmt.Sprintf("Invalid setting: %d", settings.Operation))
}

// Instantiates a new connection with given settinds
func CreateConnection(settings *ConnectionSettings) *Connection {
	c := Connection {
		sock: nil,
		peer: nil,
		Out: make(chan []byte, settings.BufferSize),
		In: make(chan []byte, settings.BufferSize),
		Settings: settings,
	}
	return &c
}

// Makes a websocket connection with the given url and sends the key to it.
// Connection process: ws connect -> OK -> Ready
// Returns when the other peer has connected.
func (c *Connection) MakeWSConnection() (*websocket.Conn, error) {
	url := c.Settings.Signaling + "/"
	if (c.Settings.Operation == 0) {
		url += "offer"
	} else {
		url += "answer"
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return conn, err
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte(c.Settings.Key))
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

// Sends an ICEcandidate to the signaling server
func (c *Connection) signalCandidate(candidate *webrtc.ICECandidate) error {
	candidateJSON := []byte(candidate.ToJSON().Candidate)
	return c.sock.WriteJSON(map[string][]byte{
		"type": []byte("candidate"),
		"ice":  candidateJSON,
	})
}

func (c *Connection) MakePeerConnection() error {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: c.Settings.STUN},
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
			if err := c.signalCandidate(candidate); err != nil {
			}
		}
	})

	peer_conn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		c.State <- state
	})

	return nil
}

// Consumes the signaling messages. This function is safe to be runned asynchronously
// Expected types:
//  offer: an SDP offer
//  answer: an SDP answer
//  candidate: ICE candidate message
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

			if err != nil {
				return err
			}
		case "answer":
			// 1. Set the offer as RemoteDescription
			if err := json.Unmarshal(message["sdp"], &sdp); err != nil {
				return err
			}

			if err := c.peer.SetRemoteDescription(sdp); err != nil {
				return err
			}
		case "candidate":
			if err := c.peer.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(message["ice"])},
			); err != nil {
				return err
			}
		default:
		}
	}
}

func (c *Connection) CreateDataChannel() (*webrtc.DataChannel, error) {
	return c.peer.CreateDataChannel("data", nil)
}

func (c *Connection) CloseAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.IsClosed == true {
		return nil
	}

	close(c.In)
	close(c.Out)
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

	c.IsClosed = true
	return nil
}

// Enables the server to send the stream to the output channel
// and to receive the incoming data in the input channel
func (c *Connection) AttachFunctionality(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		// send
		var msg []byte
		for {
			msg = <-c.In
			if err := dc.Send(msg); err != nil {
				c.CloseAll()
				return
			}
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		//receive
		c.Out <- msg.Data
	})
}

