package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func testRing(t *testing.T, active string, keys map[string]string) *Keyring {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"active": active, "keys": keys})
	k, err := ParseKeyring(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestEnvelopeBindingRotationAndBackup(t *testing.T) {
	oldKey, newKey := NewKey(), NewKey()
	k := testRing(t, "old", map[string]string{"old": oldKey})
	identity := []byte(`["company-a",1,"production","credential-a",1]`)
	plain := []byte("synthetic-test-only-bearer-unique")
	e, err := k.Encrypt(plain, identity)
	if err != nil {
		t.Fatal(err)
	}
	backup, _ := json.Marshal(e)
	if bytes.Contains(backup, plain) {
		t.Fatal("plaintext in backup")
	}
	var restored Envelope
	if json.Unmarshal(backup, &restored) != nil {
		t.Fatal("backup unreadable")
	}
	consumed := false
	if err := k.Consume(restored, identity, func(got []byte) error {
		consumed = bytes.Equal(got, plain)
		return nil
	}); err != nil || !consumed {
		t.Fatal("restore with original key failed")
	}
	// Every axis of the context, including company and environment, is AAD.
	for _, other := range []string{
		`["company-b",1,"production","credential-a",1]`,
		`["company-a",2,"production","credential-a",1]`,
		`["company-a",1,"development","credential-a",1]`,
		`["company-a",1,"production","credential-b",1]`,
		`["company-a",1,"production","credential-a",2]`,
	} {
		if err := k.Consume(restored, []byte(other), func([]byte) error {
			t.Fatal("wrong scope reached adapter")
			return nil
		}); !errors.Is(err, ErrEnvelope) {
			t.Fatal("wrong scope accepted")
		}
	}
	rotating := testRing(t, "new", map[string]string{"old": oldKey, "new": newKey})
	rewrapped, err := rotating.Rewrap(restored, identity)
	if err != nil || rewrapped.KeyID != "new" || !bytes.Equal(e.Data, rewrapped.Data) {
		t.Fatal("rewrap failed or changed data")
	}
	final := testRing(t, "new", map[string]string{"new": newKey})
	if final.Consume(rewrapped, identity, func([]byte) error { return nil }) != nil {
		t.Fatal("old key still needed")
	}
	if final.Consume(restored, identity, func([]byte) error { return nil }) == nil {
		t.Fatal("retired key opened old backup")
	}
	if _, err := final.Rewrap(restored, identity); err == nil {
		t.Fatal("missing key silently skipped")
	}
}

func TestEnvelopeTamperAndMemoryClearing(t *testing.T) {
	k := testRing(t, "v1", map[string]string{"v1": NewKey()})
	context := []byte("company/app/environment/role/endpoint/revision")
	for _, field := range []string{"data", "wrapped", "version", "key_id"} {
		e, _ := k.Encrypt([]byte("test-token"), context)
		switch field {
		case "data":
			e.Data[len(e.Data)-1] ^= 1
		case "wrapped":
			e.Wrapped[0] ^= 1
		case "version":
			e.Version++
		case "key_id":
			e.KeyID = "missing"
		}
		if err := k.Consume(e, context, func([]byte) error { t.Fatal("tamper accepted"); return nil }); err == nil {
			t.Fatal("tamper not rejected")
		}
	}
	e, _ := k.Encrypt([]byte("test-token"), context)
	var retained []byte
	sentinel := errors.New("adapter failure")
	if err := k.Consume(e, context, func(value []byte) error { retained = value; return sentinel }); err != sentinel {
		t.Fatal("adapter result lost")
	}
	if !bytes.Equal(retained, make([]byte, len(retained))) {
		t.Fatal("temporary plaintext not cleared")
	}
}

func TestKeyringFailsClosed(t *testing.T) {
	for _, raw := range []string{"", "{}", `{"active":"missing","keys":{}}`, `{"active":"a","keys":{"a":"invalid"}}`} {
		if _, err := ParseKeyring(raw); !errors.Is(err, ErrEnvelope) {
			t.Fatal("invalid keyring accepted")
		}
	}
}
