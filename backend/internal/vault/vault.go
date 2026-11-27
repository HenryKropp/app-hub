// Package vault keeps app settings that are secrets: AES-256-GCM with the
// key in HUB_SECRET_KEY. Values never leave the Hub except into the app's
// environment when it starts.
//
// Every value sealed carries the id of its key (the first 4 bytes of the
// key's SHA-256), so the key can change: values under an older key still open
// while that key is listed in HUB_SECRET_KEYS_OLD, and cmd/trocarchave
// re-seals them under the current one (Fase 0 de segurança).
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

var ErrNoKey = errors.New("cofre sem chave")

// keyHeader starts every value sealed with a key id: "ZK1" + 4-byte id.
const keyHeader = "ZK1"

type key struct {
	id   [4]byte
	aead cipher.AEAD
}

type Vault struct {
	cur key
	old []key
}

func parseKey(keyB64 string) (key, error) {
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(raw) != 32 {
		return key{}, errors.New("HUB_SECRET_KEY precisa ter 32 bytes em base64")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return key{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return key{}, err
	}
	sum := sha256.Sum256(raw)
	var k key
	copy(k.id[:], sum[:4])
	k.aead = aead
	return k, nil
}

// New opens the vault with the current key and, optionally, older keys that
// may still have sealed some values.
func New(current string, old ...string) (*Vault, error) {
	if current == "" {
		return nil, ErrNoKey
	}
	cur, err := parseKey(current)
	if err != nil {
		return nil, err
	}
	v := &Vault{cur: cur}
	for _, o := range old {
		k, err := parseKey(o)
		if err != nil {
			return nil, errors.New("HUB_SECRET_KEYS_OLD tem uma chave inválida")
		}
		if k.id != cur.id {
			v.old = append(v.old, k)
		}
	}
	return v, nil
}

// KeyID is the id of the current key.
func (v *Vault) KeyID() [4]byte { return v.cur.id }

func (v *Vault) keys() []key { return append([]key{v.cur}, v.old...) }

func (v *Vault) byID(id []byte) (key, bool) {
	for _, k := range v.keys() {
		if bytes.Equal(k.id[:], id) {
			return k, true
		}
	}
	return key{}, false
}

func (v *Vault) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, v.cur.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte(keyHeader), v.cur.id[:]...)
	out = append(out, nonce...)
	return v.cur.aead.Seal(out, nonce, []byte(plain), nil), nil
}

func (v *Vault) Open(sealed []byte) (string, error) {
	if h := len(keyHeader) + 4; len(sealed) > h && bytes.HasPrefix(sealed, []byte(keyHeader)) {
		if k, ok := v.byID(sealed[len(keyHeader):h]); ok {
			if out, err := openWith(k.aead, sealed[h:]); err == nil {
				return out, nil
			}
		}
	}
	// Values sealed before key ids: nonce||ciphertext, under any known key.
	for _, k := range v.keys() {
		if out, err := openWith(k.aead, sealed); err == nil {
			return out, nil
		}
	}
	return "", errors.New("valor guardado inválido ou de outra chave")
}

func openWith(a cipher.AEAD, sealed []byte) (string, error) {
	n := a.NonceSize()
	if len(sealed) < n {
		return "", errors.New("valor guardado inválido")
	}
	out, err := a.Open(nil, sealed[:n], sealed[n:], nil)
	return string(out), err
}

// Current reports whether a sealed value is already under the current key,
// so a key change can skip it.
func (v *Vault) Current(sealed []byte) bool {
	h := len(keyHeader) + 4
	if len(sealed) <= h || !bytes.HasPrefix(sealed, []byte(keyHeader)) || !bytes.Equal(sealed[len(keyHeader):h], v.cur.id[:]) {
		return false
	}
	_, err := openWith(v.cur.aead, sealed[h:])
	return err == nil
}

func NewKey() string {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return base64.StdEncoding.EncodeToString(k)
}
