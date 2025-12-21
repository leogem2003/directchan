package connection

import (
	"encoding/json"
	"errors"
	"sync"

	ws "github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)


// Settings for a Connection object
type ConnectionSettings struct {
	Signaling string // address of the signaling server ("ws://<ip>:<port>")
	STUN []string
	TURN string
	Key string // Channel's identifier
	BufferSize uint // Size in bytes of the output/input buffers
}

type Connection struct {
	// Signaling connection (ws)
	sock *ws.Conn
	// Peer connection (webrtc)
	peer *webrtc.PeerConnection

	// IO buffers
	// connection output (receive from remote)
	Out chan []byte
	// connection input (send to remote)
	In  chan []byte

	// State channel
	State chan webrtc.PeerConnectionState 

	// settings
	Settings *ConnectionSettings

	// true iff the connection has an offer role
	offer bool

	// true iff CloseAll has been called
	IsClosed bool

	mu sync.Mutex	
}

// Instantiates a new connection from its settings.
func FromSettings(settings *ConnectionSettings) (*Connection, error) {
	if settings.BufferSize == 0 {
		return nil, errors.New("Buffer size must be greater than 0")
	}
	c := CreateConnection(settings)
	err := c.ConnectSignaling()
	if err != nil {
		return c, err
	}

	if err := c.MakePeerConnection(); err != nil {
		c.CloseAll()
		return c, err
	}

	if c.offer {
		return Offer(c)
	} 
	return Answer(c)
}

// Instantiates a new connection with given settings
func CreateConnection(settings *ConnectionSettings) *Connection {
	c := Connection {
		sock: nil,
		peer: nil,
		Out: make(chan []byte, settings.BufferSize),
		In: make(chan []byte, settings.BufferSize),
		State: make(chan webrtc.PeerConnectionState, 1),
		Settings: settings,
	}
	return &c
}

// Makes a ws connection with the given url and sends the key to it.
// Connection process: ws connect -> (OFFER|ANSWER) -> Ready
// Returns when the other peer has connected.
func (c *Connection) ConnectSignaling() error {
	url := c.Settings.Signaling + "/"

	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		return err
	}

	err = conn.WriteMessage(ws.TextMessage, []byte(c.Settings.Key))
	if err != nil {
		conn.Close()
		return  err
	}

	// ROLE
	_, resp, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return err
	}

	switch string(resp) {
		case "OFFER": c.offer=true
		case "ANSWER": c.offer=false
		default: 
			conn.Close()
			return errors.New("Bad response: " + string(resp))
	}

	// Ready
	_, resp, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		return err
	}
	if string(resp) != "Ready" {
		conn.Close()
		return errors.New("Bad response: " + string(resp))
	}
	c.sock = conn
	return nil
}

// Sends an ICEcandidate to the signaling server
func (c *Connection) signalCandidate(candidate *webrtc.ICECandidate) error {
	candidateJSON := []byte(candidate.ToJSON().Candidate)
	c.mu.Lock()
	defer c.mu.Unlock()
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

// Connect In and Out channels to the peer connection
// Without calling this function, those channels are detached
// and will not send data
func (c *Connection) AttachFunctionality(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		// send
		for msg := range c.In {
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

// Answer to a sdp offer.
// Spawns a Connection.ConsumeSignaling process and returns
// the newly created Connection object
func Answer(connection *Connection) (*Connection, error) {
	connection.peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		connection.AttachFunctionality(dc)
	})
	go connection.ConsumeSignaling()
	return connection, nil
}

// Makes an RTC offer. 
// Spawns a Connection.ConsumeSignaling process
func Offer(connection *Connection) (*Connection, error) {
	dc, err := connection.CreateDataChannel()
	if err != nil {
		connection.CloseAll()
		return connection, err
	}
	connection.AttachFunctionality(dc)

	offer, err := connection.peer.CreateOffer(nil)
	if err != nil {
		connection.CloseAll()
		return connection, err
	}

	if err = connection.peer.SetLocalDescription(offer); err != nil {
		connection.CloseAll()
		return connection, err
	}

	offerJSON, err := json.Marshal(offer)
	if err != nil {
		connection.CloseAll()
		return connection, err
	}

	connection.sock.WriteJSON(map[string][]byte{
		"type": []byte("offer"),
		"sdp":  offerJSON,
	})


	go connection.ConsumeSignaling()
	return connection, nil
}
