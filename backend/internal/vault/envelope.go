package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
)

// ErrEnvelope deliberately carries neither input nor cryptographic errors.
var ErrEnvelope = errors.New("credential vault unavailable or envelope invalid")

// Keyring is independent of the hosted-app vault and SESSION_SECRET.
// Configure exclusively through the workspace Secret CREDENTIAL_VAULT_KEYRING.
type Keyring struct {
	active string
	keys   map[string]cipher.AEAD
}

// Envelope encrypts a random data key with a versioned master key. The caller's
// immutable company/app/environment/credential identity is authenticated twice.
type Envelope struct {
	Version int    `json:"version"`
	KeyID   string `json:"key_id"`
	Wrapped []byte `json:"wrapped"`
	Data    []byte `json:"data"`
}

func ParseKeyring(raw string) (*Keyring, error) {
	var config struct {
		Active string            `json:"active"`
		Keys   map[string]string `json:"keys"`
	}
	if json.Unmarshal([]byte(raw), &config) != nil || len(config.Keys) == 0 || len(config.Keys) > 16 {
		return nil, ErrEnvelope
	}
	k := &Keyring{active: config.Active, keys: map[string]cipher.AEAD{}}
	for id, encoded := range config.Keys {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`).MatchString(id) {
			return nil, ErrEnvelope
		}
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != 32 {
			return nil, ErrEnvelope
		}
		block, err := aes.NewCipher(key)
		clear(key)
		if err != nil {
			return nil, ErrEnvelope
		}
		k.keys[id], err = cipher.NewGCM(block)
		if err != nil {
			return nil, ErrEnvelope
		}
	}
	if k.keys[k.active] == nil {
		return nil, ErrEnvelope
	}
	return k, nil
}

func sealBound(a cipher.AEAD, value, aad []byte) ([]byte, error) {
	n := make([]byte, a.NonceSize())
	if _, err := rand.Read(n); err != nil {
		return nil, ErrEnvelope
	}
	return a.Seal(n, n, value, aad), nil
}

func openBound(a cipher.AEAD, value, aad []byte) ([]byte, error) {
	if a == nil || len(value) < a.NonceSize()+a.Overhead() {
		return nil, ErrEnvelope
	}
	out, err := a.Open(nil, value[:a.NonceSize()], value[a.NonceSize():], aad)
	if err != nil {
		return nil, ErrEnvelope
	}
	return out, nil
}

func dataCipher(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrEnvelope
	}
	return cipher.NewGCM(b)
}

func (k *Keyring) Encrypt(value, identity []byte) (Envelope, error) {
	key := make([]byte, 32)
	defer clear(key)
	if _, err := rand.Read(key); err != nil {
		return Envelope{}, ErrEnvelope
	}
	a, err := dataCipher(key)
	if err != nil {
		return Envelope{}, err
	}
	e := Envelope{Version: 1, KeyID: k.active}
	e.Data, err = sealBound(a, value, identity)
	if err == nil {
		e.Wrapped, err = sealBound(k.keys[k.active], key, identity)
	}
	return e, err
}

func (k *Keyring) unwrap(e Envelope, identity []byte) ([]byte, error) {
	if e.Version != 1 {
		return nil, ErrEnvelope
	}
	return openBound(k.keys[e.KeyID], e.Wrapped, identity)
}

// Consume never returns plaintext to its caller. Only trusted, in-process
// adapters belong in fn. There is intentionally no HTTP read-value API.
func (k *Keyring) Consume(e Envelope, identity []byte, fn func([]byte) error) error {
	key, err := k.unwrap(e, identity)
	if err != nil {
		return err
	}
	defer clear(key)
	a, err := dataCipher(key)
	if err != nil {
		return err
	}
	value, err := openBound(a, e.Data, identity)
	if err != nil {
		return err
	}
	defer clear(value)
	return fn(value)
}

func (k *Keyring) Rewrap(e Envelope, identity []byte) (Envelope, error) {
	// Authenticate the data too; do not rotate a corrupted envelope.
	if err := k.Consume(e, identity, func([]byte) error { return nil }); err != nil {
		return Envelope{}, err
	}
	key, err := k.unwrap(e, identity)
	if err != nil {
		return Envelope{}, err
	}
	defer clear(key)
	e.KeyID = k.active
	e.Wrapped, err = sealBound(k.keys[k.active], key, identity)
	return e, err
}
