package connection 

import (
	"testing"
	"slices"
)

func TestAes(t *testing.T) {
	k := CreateKey(32)
	cypher, err := NewAESGCM(k)
	if err != nil {
		t.Errorf("Error on AES instantiation: %v", err)
	}

	msg := CreateKey(64) 
	nonce := cypher.GenerateNonce()
	cyphertext, err := cypher.Encrypt(msg,nonce)
	if err != nil {
		t.Errorf("Error on encryption: %v", err)
	}

	plaintext, err := cypher.Decrypt(cyphertext,nonce)
	if err != nil {
		t.Errorf("Error on encryption: %v", err)
	}

	if !slices.Equal(plaintext, msg) {
		t.Errorf("Decrypted text and original message differ")
	}
}

func TestAESConnection(t *testing.T) {
	k := CreateKey(32)
	cypher, err := NewAESGCM(k)
	if err != nil {
		t.Errorf("Error on AES instantiation: %v", err)
	}
	
	c1 := NewDummyConnection()
	c2 := NewDummyConnection()
	c1.Connect(c2)
	aes1 := NewAESConnection(c1, cypher)
	aes2 := NewAESConnection(c2, cypher)
	go func() {
		select {
		case err := <-aes1.Err:
			t.Errorf("Error from AES 1: %f", err)
		case err := <-aes2.Err:
			t.Errorf("Error from AES 1: %f", err)
		}
	}()
	msg := []byte("heyyooo")
	aes1.Send(msg)
	recv := aes2.Recv()
	if !slices.Equal(msg, recv) {
		t.Errorf("Original text and received text differ: %s %s", string(msg), string(recv))
	} 
}
