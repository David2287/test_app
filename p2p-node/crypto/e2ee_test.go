package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/whyskydie/p2p-node/crypto"
)

func TestGenerateKey(t *testing.T) {
	kp, err := crypto.GenerateKey()
	require.NoError(t, err)
	require.NotZero(t, kp.PrivateKey)
	require.NotZero(t, kp.PublicKey)
	require.NotEqual(t, kp.PrivateKey, kp.PublicKey)
}

func TestGenerateKey_Unique(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)
	require.NotEqual(t, a.PrivateKey, b.PrivateKey)
	require.NotEqual(t, a.PublicKey, b.PublicKey)
}

func TestSharedSecret_Same(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)

	secretA := crypto.SharedSecret(a.PrivateKey, b.PublicKey)
	secretB := crypto.SharedSecret(b.PrivateKey, a.PublicKey)
	require.Equal(t, secretA, secretB)
	require.Len(t, secretA, 32)
}

func TestEncryptDecrypt(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)

	secret := crypto.SharedSecret(a.PrivateKey, b.PublicKey)
	plaintext := []byte("hello world this is a secret message")

	ciphertext, err := crypto.Encrypt(secret, plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, ciphertext)
	require.Greater(t, len(ciphertext), len(plaintext))

	decrypted, err := crypto.Decrypt(secret, ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)
	eve, err := crypto.GenerateKey()
	require.NoError(t, err)

	secretAB := crypto.SharedSecret(a.PrivateKey, b.PublicKey)
	secretEve := crypto.SharedSecret(eve.PrivateKey, a.PublicKey)

	plaintext := []byte("secret data")
	ciphertext, err := crypto.Encrypt(secretAB, plaintext)
	require.NoError(t, err)

	_, err = crypto.Decrypt(secretEve, ciphertext)
	require.Error(t, err)
	require.ErrorIs(t, err, crypto.ErrDecrypt)
}

func TestEncryptDecrypt_Empty(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)

	secret := crypto.SharedSecret(a.PrivateKey, b.PublicKey)

	ciphertext, err := crypto.Encrypt(secret, []byte{})
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(secret, ciphertext)
	require.NoError(t, err)
	require.Empty(t, decrypted)
}

func TestEncryptDecrypt_Large(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)

	secret := crypto.SharedSecret(a.PrivateKey, b.PublicKey)
	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := crypto.Encrypt(secret, plaintext)
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(secret, ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestDecrypt_Tampered(t *testing.T) {
	a, err := crypto.GenerateKey()
	require.NoError(t, err)
	b, err := crypto.GenerateKey()
	require.NoError(t, err)

	secret := crypto.SharedSecret(a.PrivateKey, b.PublicKey)
	ciphertext, err := crypto.Encrypt(secret, []byte("hello"))
	require.NoError(t, err)

	ciphertext[len(ciphertext)-1] ^= 0xFF
	_, err = crypto.Decrypt(secret, ciphertext)
	require.Error(t, err)
}

func TestMarshal_Unmarshal(t *testing.T) {
	kp, err := crypto.GenerateKey()
	require.NoError(t, err)

	privHex, pubHex := kp.Marshal()
	require.Len(t, privHex, 64)
	require.Len(t, pubHex, 64)

	restored, err := crypto.UnmarshalKey(privHex, pubHex)
	require.NoError(t, err)
	require.Equal(t, kp.PrivateKey, restored.PrivateKey)
	require.Equal(t, kp.PublicKey, restored.PublicKey)
}

func TestUnmarshal_InvalidHex(t *testing.T) {
	_, err := crypto.UnmarshalKey("zzzz", "aaaa")
	require.Error(t, err)
}

func TestUnmarshal_WrongLength(t *testing.T) {
	_, err := crypto.UnmarshalKey("aa", "bb")
	require.Error(t, err)
}
