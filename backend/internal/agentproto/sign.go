package agentproto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
)

// Agente updates are signed with an ed25519 key kept off the server (on the
// Henry's computer); the Agente carries the public half and checks every
// update before installing it, so whoever takes over the server still cannot
// push a program to the paired computers (Fase 0 de segurança, ADR 048).

// updateContext keeps an update signature from meaning anything else.
const updateContext = "ZIVAGENT1"

func updateMessage(program []byte) []byte {
	sum := sha256.Sum256(program)
	return append([]byte(updateContext), sum[:]...)
}

// SignUpdate signs the program (without any company block), in base64.
func SignUpdate(priv ed25519.PrivateKey, program []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, updateMessage(program)))
}

// VerifyUpdate checks a downloaded file (which may carry a company block at
// the end) against the signature and the Agente's public key, both base64.
// Anything missing or malformed is a refusal.
func VerifyUpdate(pubB64 string, file []byte, sigB64 string) bool {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), updateMessage(StripTrailer(file)), sig)
}

// StripTrailer returns the program without the company block the server
// appends (HubURLTrailer); a file without one comes back whole.
func StripTrailer(file []byte) []byte {
	foot := len(trailerMagic) + 4
	if len(file) < foot || !bytes.Equal(file[len(file)-foot:len(file)-4], trailerMagic) {
		return file
	}
	n := int(binary.BigEndian.Uint32(file[len(file)-4:]))
	if n == 0 || n > maxTrailerURL || n > len(file)-foot {
		return file
	}
	return file[:len(file)-foot-n]
}
