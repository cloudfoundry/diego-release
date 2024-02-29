package encryption

import (
	"crypto/cipher"
	"fmt"
	"io"
)

const NonceSize = 12

type Encrypted struct {
	Nonce      []byte
	KeyLabel   string
	CipherText []byte
}

type Encryptor interface {
	Encrypt(plaintext []byte) (Encrypted, error)
}

type Decryptor interface {
	Decrypt(encrypted Encrypted) ([]byte, error)
}

//go:generate counterfeiter -generate

//counterfeiter:generate . Cryptor

type Cryptor interface {
	Encryptor
	Decryptor
}

type cryptor struct {
	keyManager KeyManager
	prng       io.Reader
}

func NewCryptor(keyManager KeyManager, prng io.Reader) Cryptor {
	return &cryptor{
		keyManager: keyManager,
		prng:       prng,
	}
}

func (c *cryptor) Encrypt(plaintext []byte) (Encrypted, error) {
	key := c.keyManager.EncryptionKey()

	aead, err := cipher.NewGCM(key.Block())
	if err != nil {
		return Encrypted{}, fmt.Errorf("Unable to create GCM-wrapped cipher: %q", err)
	}

	nonce := make([]byte, aead.NonceSize())
	_, err = io.ReadFull(c.prng, nonce)
	if err != nil {
		return Encrypted{}, fmt.Errorf("Unable to generate random nonce: %q", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return Encrypted{KeyLabel: key.Label(), Nonce: nonce, CipherText: ciphertext}, nil
}

func (d *cryptor) Decrypt(encrypted Encrypted) ([]byte, error) {
	key := d.keyManager.DecryptionKey(encrypted.KeyLabel)
	if key == nil {
		return nil, fmt.Errorf("Key with label %q was not found", encrypted.KeyLabel)
	}

	aead, err := cipher.NewGCM(key.Block())
	if err != nil {
		return nil, fmt.Errorf("Unable to create GCM-wrapped cipher: %q", err)
	}

	return aead.Open(nil, encrypted.Nonce, encrypted.CipherText, nil)
}
