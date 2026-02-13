package connection

import (
	"slices"
	"fmt"
)

// Utility for multiplexing over peer channel

type Dispatcher struct {
	Id byte
	Connection IOChannel
	Out chan []byte
	Closed bool
}

const ZeroByte = byte(0)
const OneByte = byte(1)

//Make a new dispatcher with a certain id and cc as close channel 
func NewDispatcher(id byte, conn IOChannel) *Dispatcher {
	return &Dispatcher {
		id,
		conn,
		make(chan []byte,1),
		false,
	}
}


// Dispatch messages based on a prefix
// For each message sent, adds an initial byte with the dispatcher ID (0 or 1 for now) 
// Content sent to the first dispatcher will be received by the first dispatcher
// on the client side, and analogously for the other dispatchers.
// Spawns a goroutine forwarding dispatcher messages. The routine terminates
// when it receives an empty message or a message starting with a byte different from a dispatcher ID.
// That message is consumed and it is not forwarded to the client.
// Once the dispatch function has terminated, the dispatchers are closed.
// There is not inter-dispatcher concurrency: once a message has arrived, 
// the function blocks until the client has read the message sent to
// one of the dispatchers.
// To terminate, send to closeChannel: when the input is counsumed it is 
// guaranteed that further messages will not be consumed by the dispatcher.
// For this reason, closeChannel should have size 1.
func DualDispatch(
	conn *Connection,
	closeChannel chan bool,
) (*Dispatcher, *Dispatcher) {
	dispatcher0 := NewDispatcher(ZeroByte, conn)
	dispatcher1 := NewDispatcher(OneByte, conn)

	go func() {
		defer dispatcher0.Close()
		defer dispatcher1.Close()
		var toSend []byte
		for {
			select {
			case <-closeChannel:
				return

			case msg := <-conn.Out:
					switch len(msg) {
					case 0: return
					case 1: toSend = []byte{}
					default: toSend = msg[1:]
					}	

					switch msg[0] {
					case ZeroByte:
						dispatcher0.Out <- toSend
					case OneByte:
						dispatcher1.Out <- toSend
					default:
						return
					}
			}
		}
	}()

	return dispatcher0, dispatcher1
}

// Send b in the connection channel.
// ID is added at the beginnning of b
func (d *Dispatcher) Send(b []byte) {
	d.Connection.Send(slices.Concat([]byte{d.Id}, b))
}

// Reads the Out channel. The leading ID is 
// expected to have already been removed.
func (d *Dispatcher) Recv() []byte {
	return <-d.Out
}

// Close a dispatcher. Set closed to true and
// close Out channel. CloseChannel is left open.
// Subsequent calls will have no effects.
func (d *Dispatcher) Close() {
	if !d.Closed {
		fmt.Println("closing")
		d.Closed = true
		close(d.Out)
	}
}
