package sharedtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestConcurrentPoolSameName(t *testing.T) {
	ctx := context.Background()
	dev := os.Getenv("DATABASE_URL")
	if dev == "" {
		t.Skip("DATABASE_URL not set")
	}
	const name = "apphub_test_sharedtest"
	// Start from "does not exist" so both goroutines race the creation.
	c, err := pgx.Connect(ctx, dev)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	if _, err := c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := Pool(ctx, name)
			if err == nil {
				p.Close()
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, err := Pool(ctx, "x; DROP"); err == nil {
		t.Fatal("accepted bad name")
	}
}
