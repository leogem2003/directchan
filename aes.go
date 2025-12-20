package connection 

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

type AESGCM struct {
	aead      cipher.AEAD
	nonceSize int
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
	associatedData []byte,
) ([]byte, error) {

	if len(nonce) != c.nonceSize {
		return nil, errors.New("invalid nonce size")
	}

	// associatedData can be nil
	ciphertext := c.aead.Seal(nil, nonce, plaintext, associatedData)
	return ciphertext, nil
}

func (c *AESGCM) Decrypt(
	ciphertext []byte,
	nonce []byte,
	associatedData []byte,
) ([]byte, error) {

	if len(nonce) != c.nonceSize {
		return nil, errors.New("invalid nonce size")
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, associatedData)
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
