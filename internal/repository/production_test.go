package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestProductionRepositoryStartsWorkInTransaction(t *testing.T) {
	startedAt := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	machineID := int64(6)
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(4), "PENDIENTE", &machineID, int64(2)}},
		stateTestRow{values: []any{true, int64(2)}},
		stateTestRow{err: pgx.ErrNoRows},
		stateTestRow{values: []any{int64(1)}},
		stateTestRow{values: []any{int64(20)}},
	}}
	repository := &ProductionRepository{db: stateTestDB{}, transactions: stateTestTransactionManager{tx: tx}}
	if err := repository.StartWork(context.Background(), 9, startedAt); err != nil {
		t.Fatal(err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}

func TestProductionRepositoryFinishesWorkInTransaction(t *testing.T) {
	startedAt := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(90 * time.Minute)
	workID := int64(9)
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(4), "EN_PROCESO", int64(1), int64(6), startedAt}},
		stateTestRow{values: []any{int64(20), &workID, "TRABAJANDO"}},
		stateTestRow{values: []any{int64(21)}},
	}}
	repository := &ProductionRepository{db: stateTestDB{}, transactions: stateTestTransactionManager{tx: tx}}
	err := repository.FinishWork(context.Background(), workID, FinishWorkData{
		Result: "EXITOSO", ConsumedMaterial: "98.500", Waste: "1.500", FinishedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}
