package connection 

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
)

type AESGCM struct {
	aead      cipher.AEAD
	nonceSize int
}

type AESConnection struct {
	Conn IOChannel 
	Cypher *AESGCM	
	Err chan error
}

// NewAESGCM loads the key once and initializes AES-GCM once.
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid AES key length")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESGCM{
		aead:      aead,
		nonceSize: aead.NonceSize(),
	}, nil
}

func (c *AESGCM) NonceSize() int {
	return c.nonceSize
}

func (c *AESGCM) Encrypt(
	plaintext []byte,
	nonce []byte,
) ([]byte, error) {

	if len(nonce) != c.nonceSize {
		return nil, errors.New("invalid nonce size")
	}

	// associatedData not supported
	ciphertext := c.aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

func (c *AESGCM) Decrypt(
	ciphertext []byte,
	nonce []byte,
) ([]byte, error) {

	if len(nonce) != c.nonceSize {
		return nil, errors.New("invalid nonce size")
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (c *AESGCM) GenerateNonce() []byte {
	return CreateKey(uint32(c.nonceSize))
}

func CreateKey(length uint32) []byte {	
	key := make([]byte,length)
	rand.Read(key)
	return key
}

func ToHex(bkey []byte) string {
	return hex.EncodeToString(bkey) 
}

func FromHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}


func NewAESConnection(conn IOChannel, cypher *AESGCM) *AESConnection {
	return &AESConnection{
		conn,
		cypher,
		make(chan error, 1),
	}
}

func (c *AESConnection) Send(b []byte) {
	nonce := c.Cypher.GenerateNonce()
	msg, err := c.Cypher.Encrypt(b, nonce)
	
	if err != nil {
		c.Err <- err
	}

	c.Conn.Send(slices.Concat(msg, nonce))
}

func (c *AESConnection) Recv() []byte {
	msg := c.Conn.Recv()
	nonceOffset := len(msg)-c.Cypher.NonceSize() 
	nonce := msg[nonceOffset:]
	plaintext, err := c.Cypher.Decrypt(msg[:nonceOffset], nonce)
	if err != nil {
		c.Err <- err
	}
	return plaintext	
}

