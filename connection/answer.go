package connection;
import (
  "log"
	"github.com/pion/webrtc/v4"
)

func Answer(url string, key string) (*Connection, error) {
	connection := new(Connection)
	connection.CreateBuffers(1)
	_, err := connection.MakeWSConnection(url+"answer", key)
	log.Println("Made WS connection")
	if err != nil {
		return connection, err
	}

	if err := connection.MakePeerConnection(); err != nil {
		return connection, err
	}
	log.Println("Made peer connection")
	if err != nil {
		return connection, err
	}
	connection.peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		connection.AttachFunctionality(dc)
	})
	go connection.ConsumeSignaling()
	return connection, nil
}


