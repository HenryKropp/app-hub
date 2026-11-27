// Command agentkey keeps the Agente's release key (Fase 0 de segurança, ADR
// 048). The private key stays on the Henry's computer, never on the server;
// the Agente carries the public key and installs only updates signed with it.
//
//	agentkey gerar <arquivo>          create the key (owner-only file) and print the public key
//	agentkey publica <arquivo>        print the public key again
//	agentkey assinar <arquivo> <exe>  write <exe>.sig next to the program
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/agentproto"
)

func main() {
	args := os.Args[1:]
	var err error
	switch {
	case len(args) == 2 && args[0] == "gerar":
		var pub string
		if pub, err = generate(args[1]); err == nil {
			fmt.Println(pub)
		}
	case len(args) == 2 && args[0] == "publica":
		var pub string
		if pub, err = publicOf(args[1]); err == nil {
			fmt.Println(pub)
		}
	case len(args) == 3 && args[0] == "assinar":
		err = sign(args[1], args[2])
	default:
		err = errors.New("uso: agentkey gerar <arquivo> | publica <arquivo> | assinar <arquivo> <exe>")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agentkey:", err)
		os.Exit(1)
	}
}

// generate writes a new private key (base64 seed) to path, refusing to
// overwrite one, and returns the public key in base64.
func generate(path string) (string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("não criei a chave (já existe?): %w", err)
	}
	if _, err := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(priv.Seed())); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

func readKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("arquivo de chave inválido")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func publicOf(path string) (string, error) {
	priv, err := readKey(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)), nil
}

// sign writes exe+".sig": the release signature of the program.
func sign(keyPath, exe string) error {
	priv, err := readKey(keyPath)
	if err != nil {
		return err
	}
	program, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	return os.WriteFile(exe+".sig", []byte(agentproto.SignUpdate(priv, program)+"\n"), 0o644)
}
