package connection

import (
	"github.com/pion/webrtc/v4"
	"log/slog"
)

// Answer to a sdp offer. The signaling request is sent to
// url/answer, then the negotiation with the key follows the policy
// specified in Connectioin.MakeWSConnection.
// Spawns a Connection.ConsumeSignaling process and returns
// the newly created Connection object
func Answer(settings *ConnectionSettings) (*Connection, error) {
	connection := CreateConnection(settings)
	connection.CreateBuffers()
	_, err := connection.MakeWSConnection()
	slog.Info("Made WS connection")
	if err != nil {
		return connection, err
	}

	if err := connection.MakePeerConnection(); err != nil {
		return connection, err
	}
	slog.Info("Made peer connection")

	connection.peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		connection.AttachFunctionality(dc, "answerer")
	})
	go connection.ConsumeSignaling()
	return connection, nil
}
