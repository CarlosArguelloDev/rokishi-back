package document

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"rokishi-back/internal/models"
)

func TestQuotePDFGeneratorCreatesDocument(t *testing.T) {
	email := "cliente@example.com"
	phone := "427 123 4567"
	description := "Prototipo de carcasa"
	quote := models.Quote{
		ID: 7, CustomerName: "Empresa de prueba", CustomerEmail: &email, CustomerPhone: &phone,
		CreationDate:        time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC),
		TotalSuggestedPrice: 19675,
		Concepts: []models.QuoteConcept{{
			Description: &description, MaterialName: "PLA negro", MaterialGrams: json.Number("100"),
			DurationMinutes: 120, PieceCount: 2, SuggestedPricePerPiece: 9838, SuggestedPrice: 19675,
		}},
	}
	options := models.QuotePDFOptions{
		ValidityDays: 15, ProductionTime: "5 días hábiles", Deposit: "50 %",
		Balance: "Contra entrega", PaymentMethod: "Transferencia", Specifications: "Acabado mate.",
		DiscountBasisPoints: 0, TaxBasisPoints: 1600, TaxAmount: 3148, TotalAfterDiscountTax: 22823,
	}

	content, err := NewQuotePDFGenerator().Generate(quote, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) < 1000 {
		t.Fatalf("PDF demasiado pequeño: %d bytes", len(content))
	}
	if !strings.HasPrefix(string(content), "%PDF-") {
		t.Fatalf("encabezado PDF invalido: %q", content[:8])
	}
}

func TestQuotePDFGeneratorPaginatesLongQuotes(t *testing.T) {
	email := strings.Repeat("contacto", 20) + "@empresa.mx"
	description := strings.Repeat("Pieza personalizada ", 8)
	concepts := make([]models.QuoteConcept, 30)
	for index := range concepts {
		concepts[index] = models.QuoteConcept{
			Description: &description, MaterialName: "PETG", MaterialGrams: json.Number("125"),
			DurationMinutes: 180, PieceCount: 2, SuggestedPricePerPiece: 5000, SuggestedPrice: 10000,
		}
	}
	quote := models.Quote{
		ID: 8, CustomerName: strings.Repeat("Empresa ", 12), CustomerEmail: &email,
		CreationDate:        time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC),
		TotalSuggestedPrice: 300000, Concepts: concepts,
	}
	options := models.QuotePDFOptions{
		ValidityDays: 15, ProductionTime: strings.Repeat("Producción especial ", 5),
		Deposit: strings.Repeat("Anticipo acordado ", 4), Balance: strings.Repeat("Saldo contra entrega ", 4),
		PaymentMethod: strings.Repeat("Transferencia bancaria ", 4), TaxBasisPoints: 1600,
		TaxAmount: 48000, TotalAfterDiscountTax: 348000,
	}

	content, err := NewQuotePDFGenerator().Generate(quote, options)
	if err != nil {
		t.Fatal(err)
	}
	if pages := bytes.Count(content, []byte("/Type /Page\n")); pages < 2 {
		t.Fatalf("se esperaba un PDF de varias paginas, se detectaron %d", pages)
	}
}
