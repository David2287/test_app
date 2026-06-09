package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const NonceSize = chacha20poly1305.NonceSizeX

var ErrDecrypt = errors.New("decryption failed")

type KeyPair struct {
	PrivateKey [32]byte
	PublicKey  [32]byte
}

func GenerateKey() (*KeyPair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, fmt.Errorf("rand.Read: %w", err)
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("curve25519.ScalarBaseMult: %w", err)
	}
	var pubArr [32]byte
	copy(pubArr[:], pub)
	return &KeyPair{PrivateKey: priv, PublicKey: pubArr}, nil
}

func SharedSecret(priv, pub [32]byte) []byte {
	out, err := curve25519.X25519(priv[:], pub[:])
	if err != nil {
		return nil
	}
	return out
}

func Encrypt(sharedSecret, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("chacha20poly1305: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("rand.Read nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func Decrypt(sharedSecret, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("chacha20poly1305: %w", err)
	}
	if len(ciphertext) < aead.NonceSize() {
		return nil, ErrDecrypt
	}
	nonce, ct := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return plaintext, nil
}

func (kp *KeyPair) Marshal() (privHex, pubHex string) {
	return hex.EncodeToString(kp.PrivateKey[:]), hex.EncodeToString(kp.PublicKey[:])
}

func UnmarshalKey(privHex, pubHex string) (*KeyPair, error) {
	priv, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, fmt.Errorf("hex.DecodeString private: %w", err)
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("hex.DecodeString public: %w", err)
	}
	if len(priv) != 32 || len(pub) != 32 {
		return nil, errors.New("key length must be 32 bytes")
	}
	var kp KeyPair
	copy(kp.PrivateKey[:], priv)
	copy(kp.PublicKey[:], pub)
	return &kp, nil
}
