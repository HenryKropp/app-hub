package agentproto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

// The signature covers the program without the company block the server
// appends, so one signature serves every company's download.
func TestUpdateSignatureCoversProgramWithoutTrailer(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	program := []byte("MZ-programa-novo")
	sig := SignUpdate(priv, program)
	withTrailer := append(append([]byte{}, program...), HubURLTrailer("https://ziv.test/w/teste")...)
	for name, file := range map[string][]byte{"plain": program, "company": withTrailer} {
		if !VerifyUpdate(pubB64, file, sig) {
			t.Fatalf("%s: good signature refused", name)
		}
	}
	tampered := append([]byte{}, withTrailer...)
	tampered[2] ^= 1
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	for name, ok := range map[string]bool{
		"tampered":  VerifyUpdate(pubB64, tampered, sig),
		"no sig":    VerifyUpdate(pubB64, program, ""),
		"no key":    VerifyUpdate("", program, sig),
		"other key": VerifyUpdate(base64.StdEncoding.EncodeToString(other), program, sig),
		"bad b64":   VerifyUpdate(pubB64, program, "%%%"),
	} {
		if ok {
			t.Fatalf("%s: accepted", name)
		}
	}
	if got := StripTrailer(withTrailer); string(got) != string(program) {
		t.Fatalf("StripTrailer = %q", got)
	}
}
