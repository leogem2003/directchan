package connection

import (
	"slices"
	"testing"
)

func TestDispatcher(t *testing.T) {
	settings := &ConnectionSettings{
		Signaling:"ws://0.0.0.0:8080",
		STUN:[]string{"stun:stun.l.google.com:19302"},
		Key:"cd",
		BufferSize:1,
	}
	
	sync := make(chan bool, 1)

	p1 := []byte("A")
	p2 := []byte("B")

	go func() {
		c1, err := FromSettings(settings)
		cc := make(chan bool, 1)
		if err != nil {
			t.Errorf("Connection error: %v", err)
		}
		d1, d2 := DualDispatch(c1, cc)
		d1.Send(p1)
		r2 := d2.Recv()
		if !slices.Equal(r2,p2) {
			t.Errorf("Dispatcher 2B received %v instead of %v", r2, p2)
		}
		d2.Send(p2)
		r1 := d1.Recv()
		if !slices.Equal(r1,p1) {
			t.Errorf("Dispatcher 1B received %v instead of %v", r2, p2)
		}
		cc <- true

		r1 = c1.Recv()
		if !slices.Equal(r1,p1) {
			t.Errorf("Original connection received %v instead of %v", r1, p1)
		}
		c1.Send(p1)

		sync <-true
	}()

	c2, err := FromSettings(settings)
	if err != nil {
		t.Errorf("Connection error: %v", err)
	}
	cc := make(chan bool, 1)
	d1, d2 := DualDispatch(c2, cc)
	d2.Send(p2)
	r1 := d1.Recv()
	if !slices.Equal(r1,p1) {
		t.Errorf("Dispatcher 1A received %v instead of %v", r1, p1)
	}

	d1.Send(p1)
	r2 := d2.Recv()
	if !slices.Equal(r2,p2) {
		t.Errorf("Dispatcher 2A received %v instead of %v", r2, p2)
	}
	cc <- true
	c2.Send(p1)	

	r1 = c2.Recv()
	if !slices.Equal(r1,p1) {
		t.Errorf("Original connection received %v instead of %v", r1, p1)
	}

	<-sync
}
