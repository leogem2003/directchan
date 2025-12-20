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
	cyphertext, err := cypher.Encrypt(msg,nonce,nil)
	if err != nil {
		t.Errorf("Error on encryption: %v", err)
	}

	plaintext, err := cypher.Decrypt(cyphertext,nonce,nil)
	if err != nil {
		t.Errorf("Error on encryption: %v", err)
	}

	if !slices.Equal(plaintext, msg) {
		t.Errorf("Decrypted text and original message differ")
	}
}
