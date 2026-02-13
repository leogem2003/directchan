package connection

type DummyConnection struct {
	Out chan []byte
	Peer *DummyConnection
}

func NewDummyConnection() *DummyConnection {
	return &DummyConnection{
		make(chan []byte, 1),
		nil,
	}
}

func (c *DummyConnection) Connect(c2 *DummyConnection) {
	c.Peer = c2
	c2.Peer = c
}

func (c *DummyConnection) Send(b []byte) {
	go func(){c.Peer.Out <- b}()
}


func (c *DummyConnection) Recv() []byte {
	return <- c.Out
}
