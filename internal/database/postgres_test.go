package database

import (
	"context"
	"testing"
)

func TestNewPoolRejectsInvalidURL(t *testing.T) {
	for _, databaseURL := range []string{"not-a-url", "://invalid", "postgres://localhost"} {
		pool, err := NewPool(context.Background(), databaseURL)
		if pool != nil {
			pool.Close()
		}
		if err == nil {
			t.Errorf("NewPool(%q) should reject invalid DATABASE_URL", databaseURL)
		}
	}
}
