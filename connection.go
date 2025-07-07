package connection

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"log/slog"
	"os"
)

var Logger = slog.Default()

// Settings for a Connection object
type ConnectionSettings struct {
	Signaling string // address of the signaling server ("ws://<ip>:<port>")
	STUN []string
	TURN string
	Key string // Channel's identifier
	BufferSize uint // Size in bytes of the output/input buffers
	Operation string // ("offer" | "answer")
}

type Connection struct {
	// Signaling connection (websocket)
	sock *websocket.Conn
	// Peer connection (webrtc)
	peer *webrtc.PeerConnection

	// IO buffers
	Out chan []byte
	In  chan []byte

	// State channel
	State chan string

	// settings
	Settings *ConnectionSettings
}

// Instantiates a new connection with given settinds
func CreateConnection(settings *ConnectionSettings) *Connection {
	c := new(Connection)
	c.Settings = settings
	return c
}

// Makes a websocket connection with the given url and sends the key to it.
// Connection process: ws connect -> OK -> Ready
// Returns when the other peer has connected.
func (c *Connection) MakeWSConnection() (*websocket.Conn, error) {
	Logger.Debug("Establishing websocket connection", "signaling server: ", c.Settings.Signaling)
	url := c.Settings.Signaling + "/" + c.Settings.Operation
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		Logger.Error(err.Error())
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
			Logger.Debug("Sending ICE candidate")
			if err := c.signalCandidate(candidate); err != nil {
				Logger.Error("Failed to send ICE candidate", "Error:", err.Error())
			}
		}
	})

	peer_conn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		Logger.Debug("Connection state changed", "state:", state.String())
		if state == webrtc.PeerConnectionStateClosed {
			Logger.Debug("exiting...")
			os.Exit(0)
		} else if state == webrtc.PeerConnectionStateFailed {
			Logger.Debug("exiting on failure...")
			os.Exit(0)
		}
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

			Logger.Debug("Received offer")
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

			Logger.Debug("sent answer")
			if err != nil {
				return err
			}
		case "answer":
			// 1. Set the offer as RemoteDescription
			if err := json.Unmarshal(message["sdp"], &sdp); err != nil {
				return err
			}

			Logger.Debug("Received answer")
			if err := c.peer.SetRemoteDescription(sdp); err != nil {
				Logger.Error("Invalid remote offer", "Error: ", err.Error())
				return err
			}
		case "candidate":
			Logger.Debug("Received ICE candidate", "candidate: ", string(message["ice"]))
			if err := c.peer.AddICECandidate(
				webrtc.ICECandidateInit{Candidate: string(message["ice"])},
			); err != nil {
				return err
			}
		default:
			Logger.Debug("unknown signal:", "type", message["type"])
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

// Enables the server to send the stream to the output channel
// and to receive the incoming data in the input channel
func (c *Connection) AttachFunctionality(dc *webrtc.DataChannel, role string) {
	dc.OnOpen(func() {
		Logger.Debug("channel opened!")
		dc.SendText("Greetings from "+role)
		var msg []byte
		for {
			msg = <-c.In
			if err := dc.Send(msg); err != nil {
				Logger.Error("Error while sending message", "Error: ", err.Error())
			}
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		c.Out <- msg.Data
	})
}

func (c *Connection) CreateBuffers() error {
	if c.Settings.BufferSize == 0 {
		return errors.New("Invalid size for channel: 0")
	}
	in := make(chan []byte, c.Settings.BufferSize)
	out := make(chan []byte, c.Settings.BufferSize)
	c.In = in
	c.Out = out
	return nil
}
