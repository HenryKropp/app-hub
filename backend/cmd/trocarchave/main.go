// Command trocarchave re-seals everything the vault protects under the
// current HUB_SECRET_KEY, in every company. Use it after changing the key:
//
//  1. put the new key in HUB_SECRET_KEY and the old one in
//     HUB_SECRET_KEYS_OLD, and restart the server;
//  2. run this command with the same settings;
//  3. remove HUB_SECRET_KEYS_OLD and restart again.
//
// It is safe to run twice: values already under the current key are skipped.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/config"
	"github.com/HenryKropp/app-hub/backend/internal/keyswap"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "trocarchave:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	v, err := vault.New(cfg.SecretKey, cfg.OldSecretKeys...)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	rep, err := keyswap.Rewrap(ctx, pool, v)
	fmt.Printf("chaves de apps recifradas: %d; cópias de banco recifradas: %d\n", rep.Values, rep.Copies)
	if err != nil {
		return err
	}
	id := v.KeyID()
	fmt.Printf("tudo está na chave atual (%x). Já pode tirar HUB_SECRET_KEYS_OLD.\n", id)
	return nil
}
