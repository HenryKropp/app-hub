package vault

import (
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	v, err := New(NewKey())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := v.Seal("senha-123")
	b, _ := v.Seal("senha-123")
	if string(a) == string(b) {
		t.Fatal("same nonce twice")
	}
	if strings.Contains(string(a), "senha-123") {
		t.Fatal("plain text inside")
	}
	got, err := v.Open(a)
	if err != nil || got != "senha-123" {
		t.Fatalf("open %q %v", got, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	v1, _ := New(NewKey())
	v2, _ := New(NewKey())
	s, _ := v1.Seal("x")
	if _, err := v2.Open(s); err == nil {
		t.Fatal("opened with another key")
	}
}

func TestNoKey(t *testing.T) {
	if _, err := New(""); err != ErrNoKey {
		t.Fatalf("err %v", err)
	}
	if _, err := New("curta"); err == nil {
		t.Fatal("accepted short key")
	}
}

func TestTamperedFails(t *testing.T) {
	v, _ := New(NewKey())
	s, _ := v.Seal("x")
	s[len(s)-1] ^= 1
	if _, err := v.Open(s); err == nil {
		t.Fatal("opened tampered")
	}
}
