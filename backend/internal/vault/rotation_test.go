package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"
)

// legacySeal writes the format used before key ids existed (nonce||ct).
func legacySeal(t *testing.T, keyB64, plain string) []byte {
	t.Helper()
	key, _ := base64.StdEncoding.DecodeString(keyB64)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)
	return aead.Seal(nonce, nonce, []byte(plain), nil)
}

func TestSealCarriesKeyID(t *testing.T) {
	a := NewKey()
	v, err := New(a)
	if err != nil {
		t.Fatal(err)
	}
	sealed, _ := v.Seal("segredo")
	id := v.KeyID()
	if !bytes.HasPrefix(sealed, append([]byte(keyHeader), id[:]...)) {
		t.Fatalf("sealed value lacks header+id: %x", sealed[:8])
	}
	if !v.Current(sealed) {
		t.Fatal("fresh value must be current")
	}
}

func TestOpenLegacyAndOldKeys(t *testing.T) {
	a, b := NewKey(), NewKey()
	old := legacySeal(t, a, "antigo")
	va, _ := New(a)
	byA, _ := va.Seal("de A")

	vb, err := New(b, a) // current B, A still accepted
	if err != nil {
		t.Fatal(err)
	}
	for sealed, want := range map[string]string{string(old): "antigo", string(byA): "de A"} {
		got, err := vb.Open([]byte(sealed))
		if err != nil || got != want {
			t.Fatalf("open = %q, %v; want %q", got, err, want)
		}
		if vb.Current([]byte(sealed)) {
			t.Fatal("value under the old key must not be current")
		}
	}
	vOnlyB, _ := New(b)
	if _, err := vOnlyB.Open(byA); err == nil {
		t.Fatal("value under A must not open without A")
	}
	if _, err := New(b, "curta"); err == nil {
		t.Fatal("invalid old key must be refused")
	}
}

func TestStreamKeyIDAndOldKeys(t *testing.T) {
	a, b := NewKey(), NewKey()
	va, _ := New(a)
	var byA bytes.Buffer
	w, _ := va.SealStream(&byA)
	w.Write(bytes.Repeat([]byte("x"), chunkSize+10))
	w.Close()
	if !Sealed(byA.Bytes()) || va.StreamCurrent(byA.Bytes()) != true {
		t.Fatal("new stream must be sealed and current")
	}
	vb, _ := New(b, a)
	if vb.StreamCurrent(byA.Bytes()) {
		t.Fatal("stream under A is not current for B")
	}
	r, err := vb.OpenStream(bytes.NewReader(byA.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil || len(got) != chunkSize+10 {
		t.Fatalf("read %d, %v", len(got), err)
	}
}

// Streams written before key ids (AHVAULT1) still open, with any known key.
func TestLegacyStreamOpens(t *testing.T) {
	a, b := NewKey(), NewKey()
	va, _ := New(a)
	var legacy bytes.Buffer
	legacy.WriteString(legacyStreamMagic)
	sw := &sealWriter{v: va, aead: va.cur.aead, w: &legacy, buf: make([]byte, 0, chunkSize)}
	sw.Write([]byte("cópia antiga"))
	sw.Close()
	vb, _ := New(b, a)
	if !Sealed(legacy.Bytes()) {
		t.Fatal("legacy stream must count as sealed")
	}
	r, err := vb.OpenStream(bytes.NewReader(legacy.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := io.ReadAll(r); err != nil || string(got) != "cópia antiga" {
		t.Fatalf("legacy stream = %q, %v", got, err)
	}
}
