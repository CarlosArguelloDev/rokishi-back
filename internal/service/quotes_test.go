package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeQuoteRepository struct {
	data repository.QuoteData
	err  error
}

func (f fakeQuoteRepository) GetCalculationData(context.Context, int64, int64) (repository.QuoteData, error) {
	return f.data, f.err
}

func decimalPointer(value string) *string { return &value }

func validQuoteData() repository.QuoteData {
	return repository.QuoteData{
		MachineID:         1,
		MachineActive:     true,
		LocationID:        1,
		PowerWatts:        decimalPointer("350.00"),
		MaterialID:        2,
		MaterialActive:    true,
		MaterialCostPerKG: "400.00",
		InternalCostHour:  decimalPointer("25.00"),
		SalePriceHour:     decimalPointer("70.00"),
		PreparationCost:   decimalPointer("15.00"),
		EnergyCostPerKWh:  decimalPointer("2.5000"),
	}
}

func TestQuoteServiceCalculatesKnownResult(t *testing.T) {
	quoteService := NewQuoteService(fakeQuoteRepository{data: validQuoteData()})
	calculation, err := quoteService.Calculate(context.Background(), CalculateQuoteInput{
		MachineID: 1, MaterialID: 2, MaterialGrams: "100", DurationMinutes: 120, PieceCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := models.QuoteCalculation{
		MachineID: 1, MaterialID: 2, MaterialGrams: json.Number("100"), DurationMinutes: 120, PieceCount: 2,
		MaterialCost: 4000, MachineCost: 5000, ElectricityCost: 175, PreparationCost: 1500,
		Subtotal: 10675, SuggestedPrice: 19675, SuggestedPricePerPiece: 9838,
	}
	if calculation != want {
		t.Fatalf("calculation = %+v, want %+v", calculation, want)
	}
}

func TestQuoteServiceRejectsInvalidInput(t *testing.T) {
	quoteService := NewQuoteService(fakeQuoteRepository{data: validQuoteData()})
	inputs := []CalculateQuoteInput{
		{MaterialID: 2, MaterialGrams: "100", DurationMinutes: 60, PieceCount: 1},
		{MachineID: 1, MaterialGrams: "100", DurationMinutes: 60, PieceCount: 1},
		{MachineID: 1, MaterialID: 2, MaterialGrams: "0", DurationMinutes: 60, PieceCount: 1},
		{MachineID: 1, MaterialID: 2, MaterialGrams: "1.0001", DurationMinutes: 60, PieceCount: 1},
		{MachineID: 1, MaterialID: 2, MaterialGrams: "100", PieceCount: 1},
		{MachineID: 1, MaterialID: 2, MaterialGrams: "100", DurationMinutes: 60},
	}
	for _, input := range inputs {
		if _, err := quoteService.Calculate(context.Background(), input); !isValidationError(err) {
			t.Fatalf("expected validation error for %+v, got %v", input, err)
		}
	}
}

func TestQuoteServiceRequiresActiveConfiguredResources(t *testing.T) {
	tests := []struct {
		name       string
		edit       func(*repository.QuoteData)
		validation bool
	}{
		{"inactive machine", func(data *repository.QuoteData) { data.MachineActive = false }, true},
		{"inactive material", func(data *repository.QuoteData) { data.MaterialActive = false }, true},
		{"missing power", func(data *repository.QuoteData) { data.PowerWatts = nil }, false},
		{"missing machine rate", func(data *repository.QuoteData) { data.InternalCostHour = nil }, false},
		{"missing energy rate", func(data *repository.QuoteData) { data.EnergyCostPerKWh = nil }, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := validQuoteData()
			test.edit(&data)
			quoteService := NewQuoteService(fakeQuoteRepository{data: data})
			_, err := quoteService.Calculate(context.Background(), CalculateQuoteInput{
				MachineID: 1, MaterialID: 2, MaterialGrams: "100", DurationMinutes: 60, PieceCount: 1,
			})
			var configurationError *MissingConfigurationError
			if test.validation && !isValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if !test.validation && !errors.As(err, &configurationError) {
				t.Fatalf("expected missing configuration error, got %v", err)
			}
		})
	}
}

func TestQuoteServiceMapsMissingResource(t *testing.T) {
	quoteService := NewQuoteService(fakeQuoteRepository{err: repository.ErrNotFound})
	_, err := quoteService.Calculate(context.Background(), CalculateQuoteInput{
		MachineID: 1, MaterialID: 2, MaterialGrams: "100", DurationMinutes: 60, PieceCount: 1,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
