package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeQuoteRepository struct {
	data           repository.QuoteData
	quote          models.Quote
	quotes         []models.Quote
	statuses       []models.QuoteStatus
	created        repository.CreateQuoteData
	customerExists bool
	changedStatus  string
	err            error
}

type fakeQuotePDFGenerator struct {
	quote   models.Quote
	options models.QuotePDFOptions
	content []byte
	err     error
}

func (f *fakeQuotePDFGenerator) Generate(quote models.Quote, options models.QuotePDFOptions) ([]byte, error) {
	f.quote = quote
	f.options = options
	return f.content, f.err
}

func (f *fakeQuoteRepository) GetCalculationData(context.Context, int64, int64) (repository.QuoteData, error) {
	return f.data, f.err
}

func (f *fakeQuoteRepository) ActiveCustomerExists(context.Context, int64) (bool, error) {
	return f.customerExists, f.err
}

func (f *fakeQuoteRepository) Create(_ context.Context, data repository.CreateQuoteData) (models.Quote, error) {
	f.created = data
	return f.quote, f.err
}

func (f *fakeQuoteRepository) List(context.Context, repository.QuoteFilters) ([]models.Quote, error) {
	return f.quotes, f.err
}

func (f *fakeQuoteRepository) Get(context.Context, int64) (models.Quote, error) {
	return f.quote, f.err
}

func (f *fakeQuoteRepository) ListStatuses(context.Context) ([]models.QuoteStatus, error) {
	return f.statuses, f.err
}

func (f *fakeQuoteRepository) ChangeStatus(_ context.Context, _ int64, _ int64, status string) (models.Quote, error) {
	f.changedStatus = status
	return models.Quote{ID: f.quote.ID, StatusCode: status}, f.err
}

func decimalPointer(value string) *string { return &value }

func validQuoteData() repository.QuoteData {
	return repository.QuoteData{
		MachineID:         1,
		MachineCode:       "IMP-01",
		MachineName:       "Prusa MK4",
		MachineActive:     true,
		LocationID:        1,
		PowerWatts:        decimalPointer("350.00"),
		MaterialID:        2,
		MaterialName:      "PLA negro",
		MaterialActive:    true,
		MaterialCostPerKG: "400.00",
		InternalCostHour:  decimalPointer("25.00"),
		SalePriceHour:     decimalPointer("70.00"),
		PreparationCost:   decimalPointer("15.00"),
		EnergyCostPerKWh:  decimalPointer("2.5000"),
	}
}

func TestQuoteServiceCalculatesKnownResult(t *testing.T) {
	quoteService := NewQuoteService(&fakeQuoteRepository{data: validQuoteData()})
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
	quoteService := NewQuoteService(&fakeQuoteRepository{data: validQuoteData()})
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
			quoteService := NewQuoteService(&fakeQuoteRepository{data: data})
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
	quoteService := NewQuoteService(&fakeQuoteRepository{err: repository.ErrNotFound})
	_, err := quoteService.Calculate(context.Background(), CalculateQuoteInput{
		MachineID: 1, MaterialID: 2, MaterialGrams: "100", DurationMinutes: 60, PieceCount: 1,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestQuoteServiceCreatesPersistentSnapshot(t *testing.T) {
	repository := &fakeQuoteRepository{
		data: validQuoteData(), customerExists: true,
		quote: models.Quote{ID: 7, CustomerID: 3, StatusCode: "BORRADOR"},
	}
	quoteService := NewQuoteService(repository)
	quote, err := quoteService.Create(context.Background(), CreateQuoteInput{
		CustomerID: 3,
		Concepts: []CreateQuoteConceptInput{{
			Description: decimalPointer("Pieza principal"), MachineID: 1, MaterialID: 2,
			MaterialGrams: "100", DurationMinutes: 120, PieceCount: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quote.ID != 7 || len(repository.created.Concepts) != 1 {
		t.Fatalf("unexpected quote or snapshot: quote=%+v snapshot=%+v", quote, repository.created)
	}
	if repository.created.TotalCost != 10675 || repository.created.TotalSuggestedPrice != 19675 {
		t.Fatalf("unexpected totals: %+v", repository.created)
	}
	snapshot := repository.created.Concepts[0]
	if snapshot.Rates.MachineCode != "IMP-01" || snapshot.Rates.MaterialCostPerKG != "400.00" || snapshot.Calculation.SuggestedPrice != 19675 {
		t.Fatalf("unexpected concept snapshot: %+v", snapshot)
	}
}

func TestQuoteServiceValidatesCustomerAndConcepts(t *testing.T) {
	quoteService := NewQuoteService(&fakeQuoteRepository{data: validQuoteData()})
	if _, err := quoteService.Create(context.Background(), CreateQuoteInput{CustomerID: 3}); !isValidationError(err) {
		t.Fatalf("expected missing customer validation, got %v", err)
	}
	quoteService = NewQuoteService(&fakeQuoteRepository{data: validQuoteData(), customerExists: true})
	if _, err := quoteService.Create(context.Background(), CreateQuoteInput{CustomerID: 3}); !isValidationError(err) {
		t.Fatalf("expected missing concepts validation, got %v", err)
	}
}

func TestQuoteServiceControlsStatusTransitions(t *testing.T) {
	repository := &fakeQuoteRepository{quote: models.Quote{ID: 7, StatusID: 1, StatusCode: "BORRADOR"}}
	quoteService := NewQuoteService(repository)
	updated, err := quoteService.ChangeStatus(context.Background(), 7, "enviada")
	if err != nil {
		t.Fatal(err)
	}
	if updated.StatusCode != "ENVIADA" || repository.changedStatus != "ENVIADA" {
		t.Fatalf("unexpected status update: %+v", updated)
	}
	if _, err := quoteService.ChangeStatus(context.Background(), 7, "ACEPTADA"); !isValidationError(err) {
		t.Fatalf("expected invalid direct transition, got %v", err)
	}
}

func TestQuoteServiceGeneratesPDFWithDiscountAndTax(t *testing.T) {
	expiration := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	repository := &fakeQuoteRepository{quote: models.Quote{
		ID: 7, CustomerName: "Taller Norte", TotalSuggestedPrice: 10000,
		CreationDate: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC), ExpirationDate: &expiration,
	}}
	generator := &fakeQuotePDFGenerator{content: []byte("%PDF-test")}
	quoteService := NewQuoteService(repository, generator)

	document, err := quoteService.GeneratePDF(context.Background(), 7, GenerateQuotePDFInput{
		ProductionTime: "5 días hábiles", DiscountPercentage: "10", TaxPercentage: "16",
		Deposit: "50 %", Balance: "Contra entrega", PaymentMethod: "Transferencia",
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.Filename != "cotizacion-COT-000007.pdf" || string(document.Content) != "%PDF-test" {
		t.Fatalf("documento inesperado: %+v", document)
	}
	if generator.options.ValidityDays != 15 || generator.options.DiscountAmount != 1000 ||
		generator.options.TaxAmount != 1440 || generator.options.TotalAfterDiscountTax != 10440 {
		t.Fatalf("totales del PDF inesperados: %+v", generator.options)
	}
}

func TestQuoteServiceRejectsInvalidPDFPercentage(t *testing.T) {
	repository := &fakeQuoteRepository{quote: models.Quote{ID: 7, TotalSuggestedPrice: 10000}}
	quoteService := NewQuoteService(repository, &fakeQuotePDFGenerator{})
	_, err := quoteService.GeneratePDF(context.Background(), 7, GenerateQuotePDFInput{DiscountPercentage: "100.01"})
	if !isValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
