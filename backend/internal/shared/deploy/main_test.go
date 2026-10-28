package deploy_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/shared/deploy/deploytest"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	var err error
	pool, err = deploytest.Pool(context.Background(), "apphub_test_deploy")
	if err != nil {
		panic(err)
	}
	if pool == nil {
		os.Exit(0) // sem banco: os testes deste pacote não rodam
	}
	os.Exit(m.Run())
}

func ptr[T any](v T) *T { return &v }
