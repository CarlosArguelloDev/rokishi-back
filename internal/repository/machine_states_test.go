package repository

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stateTestDB struct{}

func (stateTestDB) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (stateTestDB) QueryRow(context.Context, string, ...any) pgx.Row        { return stateTestRow{} }

type stateTestRow struct {
	values []any
	err    error
}

func (r stateTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		target := reflect.ValueOf(dest[index]).Elem()
		if value == nil {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		target.Set(reflect.ValueOf(value))
	}
	return nil
}

type stateTestTransaction struct {
	rows        []scanner
	execError   error
	commitError error
	committed   bool
	rolledBack  bool
}

func (tx *stateTestTransaction) QueryRow(context.Context, string, ...any) scanner {
	row := tx.rows[0]
	tx.rows = tx.rows[1:]
	return row
}

func (tx *stateTestTransaction) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), tx.execError
}

func (tx *stateTestTransaction) Commit(context.Context) error {
	if tx.commitError == nil {
		tx.committed = true
	}
	return tx.commitError
}

func (tx *stateTestTransaction) Rollback(context.Context) error {
	if !tx.committed {
		tx.rolledBack = true
	}
	return nil
}

type stateTestTransactionManager struct {
	tx  stateTransaction
	err error
}

func (m stateTestTransactionManager) Begin(context.Context) (stateTransaction, error) {
	return m.tx, m.err
}

func statePeriodRow(id, machineID, stateID int64, code, name string, startedAt time.Time) scanner {
	return stateTestRow{values: []any{id, machineID, stateID, code, name, startedAt, nil, nil}}
}

func TestMachineStateRepositoryChangeCommitsTransaction(t *testing.T) {
	start := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	previousStart := start.Add(-time.Hour)
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(7)}},
		stateTestRow{values: []any{int64(2), "TRABAJANDO", "Trabajando", nil}},
		statePeriodRow(8, 7, 1, "DISPONIBLE", "Disponible", previousStart),
		statePeriodRow(9, 7, 2, "TRABAJANDO", "Trabajando", start),
	}}
	repository := &MachineStateRepository{db: stateTestDB{}, transactions: stateTestTransactionManager{tx: tx}}
	change, err := repository.Change(context.Background(), 7, 2, start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
	if change.Previous == nil || change.Previous.ID != 8 || change.Current.ID != 9 {
		t.Fatalf("unexpected change: %+v", change)
	}
}

func TestMachineStateRepositoryRollsBackOnInsertFailure(t *testing.T) {
	start := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	insertError := errors.New("insert failed")
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(7)}},
		stateTestRow{values: []any{int64(2), "TRABAJANDO", "Trabajando", nil}},
		statePeriodRow(8, 7, 1, "DISPONIBLE", "Disponible", start.Add(-time.Hour)),
		stateTestRow{err: insertError},
	}}
	repository := &MachineStateRepository{db: stateTestDB{}, transactions: stateTestTransactionManager{tx: tx}}
	if _, err := repository.Change(context.Background(), 7, 2, start, nil); !errors.Is(err, insertError) {
		t.Fatalf("expected insert error, got %v", err)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}

func TestMachineStateRepositoryRejectsInvalidTransition(t *testing.T) {
	start := time.Date(2026, 9, 26, 11, 0, 0, 0, time.UTC)
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(7)}},
		stateTestRow{values: []any{int64(2), "TRABAJANDO", "Trabajando", nil}},
		statePeriodRow(8, 7, 1, "DISPONIBLE", "Disponible", start),
	}}
	repository := &MachineStateRepository{db: stateTestDB{}, transactions: stateTestTransactionManager{tx: tx}}
	if _, err := repository.Change(context.Background(), 7, 2, start, nil); !errors.Is(err, ErrInvalidStateTime) {
		t.Fatalf("expected invalid transition time, got %v", err)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}
