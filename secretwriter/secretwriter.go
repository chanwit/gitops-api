package secretwriter

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/google/go-github/v31/github"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
)

const publicKeyLength = 32
const nonceLength = 24

// SecretWriter provides the ability to create and update Github secrets.
type SecretWriter struct {
	client *github.Client
}

func New(client *github.Client) SecretWriter {
	return SecretWriter{client}
}

// Write encrypts and writes a Github secret to Github using the API.
func (s SecretWriter) Write(owner, repo, name string, value []byte) (string, error) {
	publicKeyId, publicKey, err := s.getPublicKey(owner, repo)
	if err != nil {
		return "", err
	}

	encryptedValue, err := encryptValue(value, publicKey)
	if err != nil {
		return "", err
	}

	res, err := s.client.Actions.CreateOrUpdateSecret(
		context.Background(),
		owner,
		repo,
		&github.EncryptedSecret{
			Name:           name,
			KeyID:          publicKeyId,
			EncryptedValue: base64.StdEncoding.EncodeToString(encryptedValue),
		})
	if err != nil {
		return "", err
	}

	return res.Status, nil
}

func (s SecretWriter) getPublicKey( owner, repo string ) (string, *[publicKeyLength]byte, error) {
	pk, _, err := s.client.Actions.GetPublicKey(
		context.Background(),
		owner,
		repo)
	if err != nil {
		return "", nil, err
	}

	keyId := pk.GetKeyID()
	key64String := pk.GetKey()

	keySlice, err := base64.StdEncoding.DecodeString(key64String)

	var publicKey = &[32]byte{}
	copy(publicKey[:], keySlice)
	if err != nil {
		return "", nil, err
	}

	return keyId, publicKey, nil
}

func createNonce(ePublicKey, publicKey *[publicKeyLength]byte) *[nonceLength]byte {
	h, _ := blake2b.New(nonceLength, nil)
	h.Write(ePublicKey[:])
	h.Write(publicKey[:])
	var nonce = &[nonceLength]byte{}
	copy(nonce[:], h.Sum(nil))

	return nonce
}

func encryptValue(value []byte, publicKey *[publicKeyLength]byte) ([]byte, error) {
	ePublicKey, ePrivateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	crypted := make([]byte, 0, box.Overhead+len(value))

	nonce := createNonce(ePublicKey, publicKey)

	_ = box.Seal(crypted, value, nonce, publicKey, ePrivateKey)

	return append(ePublicKey[:], crypted[0:cap(crypted)]...), nil
}