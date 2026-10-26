// Package vault keeps app settings that are secrets: AES-256-GCM with the
// key in HUB_SECRET_KEY. Values never leave the Hub except into the app's
// environment when it starts.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

var ErrNoKey = errors.New("cofre sem chave")

type Vault struct{ aead cipher.AEAD }

func New(keyB64 string) (*Vault, error) {
	if keyB64 == "" {
		return nil, ErrNoKey
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != 32 {
		return nil, errors.New("HUB_SECRET_KEY precisa ter 32 bytes em base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (v *Vault) Open(sealed []byte) (string, error) {
	n := v.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("valor guardado inválido")
	}
	out, err := v.aead.Open(nil, sealed[:n], sealed[n:], nil)
	return string(out), err
}

func NewKey() string {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return base64.StdEncoding.EncodeToString(k)
}
