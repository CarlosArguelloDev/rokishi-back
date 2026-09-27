package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateError(t *testing.T) {
	if err := translateError(pgx.ErrNoRows); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := translateError(&pgconn.PgError{Code: "23505"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := translateError(&pgconn.PgError{Code: "23503"}); !errors.Is(err, ErrReferenceMissing) {
		t.Fatalf("expected missing reference, got %v", err)
	}
	original := errors.New("database unavailable")
	if err := translateError(original); !errors.Is(err, original) {
		t.Fatalf("expected original error, got %v", err)
	}
}
