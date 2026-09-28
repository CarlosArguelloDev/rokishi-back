package document

import (
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
