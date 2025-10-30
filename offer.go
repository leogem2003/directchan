package connection
import (
  "encoding/json"
)

// Makes an RTC offer. The key is sent to url/offer, 
// then the negotiation with the key follow the policy
// specified in Connectioin.MakeWSConnection.
// Spawns a Connection.ConsumeSIgnaling process
func Offer(settings *ConnectionSettings) (*Connection, error) {
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


