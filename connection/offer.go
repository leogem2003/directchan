package connection
import (
  "log"
  "encoding/json"
)

func Offer(url string, key string) (*Connection, error) {
	connection := new(Connection)

	connection.CreateBuffers(1)
	_, err := connection.MakeWSConnection(url+"offer", key)
	if err != nil {
		return connection, err
	}
	log.Println("Making WS connection")
	if err := connection.MakePeerConnection(); err != nil {
		return connection, err

	}
	log.Println("Made WS connection")

	dc, err := connection.CreateDataChannel()
	if err != nil {
		return connection, err
	}
	connection.AttachFunctionality(dc)

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


