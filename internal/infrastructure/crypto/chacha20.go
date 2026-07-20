package crypto

import (
	"crypto/rand"
	"golang.org/x/crypto/chacha20poly1305"
	"io"
)

func EncryptChaCha20(key []byte, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}
