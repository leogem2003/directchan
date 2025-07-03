package connection
import (
  "log"
  "encoding/json"
)

func Offer(settings *ConnectionSettings) (*Connection, error) {
  /*
    Makes an RTC offer. The key is sent to url/offer, 
    then the negotiation with the key follow the policy
	  specified in Connectioin.MakeWSConnection.
	  Spawns a Connection.ConsumeSIgnaling process
  */
	connection := CreateConnection(settings) 
	connection.CreateBuffers()
	log.Println("Making WS connection")
	_, err := connection.MakeWSConnection()
	if err != nil {
		return connection, err
	}
	log.Println("Made WS connection")

	if err := connection.MakePeerConnection(); err != nil {
		return connection, err
	}

	dc, err := connection.CreateDataChannel()
	if err != nil {
		return connection, err
	}
	connection.AttachFunctionality(dc, "offerer")

	offer, err := connection.peer.CreateOffer(nil)
	if err != nil {
		return connection, err
	}
	if err = connection.peer.SetLocalDescription(offer); err != nil {
		return connection, err
	}

	offerJSON, err := json.Marshal(offer)
	if err != nil {
		return connection, err
	}
	connection.sock.WriteJSON(map[string][]byte{
		"type": []byte("offer"),
		"sdp":  offerJSON,
	})

	log.Println("Sent offer")

	go connection.ConsumeSignaling()
	return connection, nil
}


