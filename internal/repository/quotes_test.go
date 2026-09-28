package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"rokishi-back/internal/models"
)

type quoteTestTransactionManager struct {
	tx quoteTransaction
}

func (m quoteTestTransactionManager) Begin(context.Context) (quoteTransaction, error) {
	return m.tx, nil
}

func quoteSnapshotData() CreateQuoteData {
	power, internal, sale, preparation, energy := "350.00", "25.00", "70.00", "15.00", "2.5000"
	return CreateQuoteData{
		CustomerID: 3, TotalCost: 10675, TotalSuggestedPrice: 19675,
		Concepts: []QuoteConceptSnapshot{{
			Calculation: models.QuoteCalculation{
				MachineID: 1, MaterialID: 2, MaterialGrams: json.Number("100"),
				DurationMinutes: 120, PieceCount: 2, MaterialCost: 4000, MachineCost: 5000,
				ElectricityCost: 175, PreparationCost: 1500, Subtotal: 10675,
				SuggestedPrice: 19675, SuggestedPricePerPiece: 9838,
			},
			Rates: QuoteData{
				MachineID: 1, MachineCode: "IMP-01", MachineName: "Prusa MK4", PowerWatts: &power,
				MaterialID: 2, MaterialName: "PLA negro", MaterialCostPerKG: "400.00",
				InternalCostHour: &internal, SalePriceHour: &sale, PreparationCost: &preparation,
				EnergyCostPerKWh: &energy,
			},
		}},
	}
}

func TestQuoteRepositoryCreateCommitsCompleteSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(7), int64(3), "Taller Norte", nil, nil, int64(1), "BORRADOR", "Borrador", nil, nil, int64(10675), int64(19675), now, now}},
		stateTestRow{values: []any{int64(11), now}},
	}}
	repository := &QuoteRepository{db: stateTestDB{}, transactions: quoteTestTransactionManager{tx: tx}}
	quote, err := repository.Create(context.Background(), quoteSnapshotData())
	if err != nil {
		t.Fatal(err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
	if quote.ID != 7 || len(quote.Concepts) != 1 || quote.Concepts[0].ID != 11 {
		t.Fatalf("unexpected persisted quote: %+v", quote)
	}
	if quote.Concepts[0].MaterialCostPerKG.String() != "400.00" || quote.Concepts[0].SuggestedPrice != 19675 {
		t.Fatalf("snapshot was not preserved: %+v", quote.Concepts[0])
	}
}

func TestQuoteRepositoryCreateRollsBackOnConceptFailure(t *testing.T) {
	now := time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
	insertError := errors.New("concept insert failed")
	tx := &stateTestTransaction{rows: []scanner{
		stateTestRow{values: []any{int64(7), int64(3), "Taller Norte", nil, nil, int64(1), "BORRADOR", "Borrador", nil, nil, int64(10675), int64(19675), now, now}},
		stateTestRow{err: insertError},
	}}
	repository := &QuoteRepository{db: stateTestDB{}, transactions: quoteTestTransactionManager{tx: tx}}
	if _, err := repository.Create(context.Background(), quoteSnapshotData()); !errors.Is(err, insertError) {
		t.Fatalf("expected concept insert error, got %v", err)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("unexpected transaction state: committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}
