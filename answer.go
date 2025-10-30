package connection

import (
	"github.com/pion/webrtc/v4"
)

// Answer to a sdp offer. The signaling request is sent to
// url/answer, then the negotiation with the key follows the policy
// specified in Connectioin.MakeWSConnection.
// Spawns a Connection.ConsumeSignaling process and returns
// the newly created Connection object
func Answer(settings *ConnectionSettings) (*Connection, error) {
	connection := CreateConnection(settings)
	_, err := connection.MakeWSConnection()
	if err != nil {
		connection.CloseAll()
		return connection, err
	}

	if err := connection.MakePeerConnection(); err != nil {
		connection.CloseAll()
		return connection, err
	}

	connection.peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		connection.AttachFunctionality(dc)
	})
	go connection.ConsumeSignaling()
	return connection, nil
}
