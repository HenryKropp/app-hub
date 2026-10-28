package vault

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

func sealAll(t *testing.T, v *Vault, plain []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w, err := v.SealStream(&out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func openAll(v *Vault, sealed []byte) ([]byte, error) {
	r, err := v.OpenStream(bytes.NewReader(sealed))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func TestStreamRoundTrip(t *testing.T) {
	v, _ := New(NewKey())
	big := make([]byte, 3*chunkSize+123)
	_, _ = rand.Read(big)
	cases := []struct {
		name  string
		plain []byte
	}{
		{"empty", nil},
		{"small", []byte("senha-123 e dados do banco")},
		{"exactly one chunk", big[:chunkSize]},
		{"several chunks", big},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sealed := sealAll(t, v, c.plain)
			if !Sealed(sealed) {
				t.Fatal("no header")
			}
			if len(c.plain) > 8 && bytes.Contains(sealed, c.plain[:8]) {
				t.Fatal("plain text inside")
			}
			got, err := openAll(v, sealed)
			if err != nil || !bytes.Equal(got, c.plain) {
				t.Fatalf("open: %v, %d bytes, want %d", err, len(got), len(c.plain))
			}
		})
	}
}

func TestStreamRejectsTampering(t *testing.T) {
	v, _ := New(NewKey())
	big := make([]byte, 2*chunkSize+10)
	sealed := sealAll(t, v, big)
	firstChunk := len(streamMagic) + 4 + v.aead.NonceSize() + chunkSize + v.aead.Overhead()

	other, _ := New(NewKey())
	flipped := append([]byte{}, sealed...)
	flipped[len(flipped)-1] ^= 1
	cases := []struct {
		name string
		v    *Vault
		data []byte
	}{
		{"cut after a whole chunk", v, sealed[:firstChunk]},
		{"cut in the middle", v, sealed[:len(sealed)/2]},
		{"one bit changed", v, flipped},
		{"extra bytes at the end", v, append(append([]byte{}, sealed...), 0)},
		{"another key", other, sealed},
		{"not sealed", v, []byte("PGDMP plain copy")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := openAll(c.v, c.data); !errors.Is(err, ErrStreamCut) {
				t.Fatalf("got %v, want ErrStreamCut", err)
			}
		})
	}
}

func TestStreamErrorsStick(t *testing.T) {
	v, _ := New(NewKey())
	sealed := sealAll(t, v, make([]byte, 2*chunkSize))
	sealed[len(streamMagic)+10] ^= 1
	r, err := v.OpenStream(bytes.NewReader(sealed))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 10)
	for i := 0; i < 2; i++ {
		if _, err := r.Read(buf); !errors.Is(err, ErrStreamCut) {
			t.Fatalf("read %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	w, _ := v.SealStream(&out)
	w.Close()
	if _, err := w.Write([]byte("x")); err == nil {
		t.Fatal("write after close accepted")
	}
}
