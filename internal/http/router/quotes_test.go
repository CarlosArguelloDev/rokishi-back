package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeQuoteService struct{}

func (fakeQuoteService) Calculate(_ context.Context, input service.CalculateQuoteInput) (models.QuoteCalculation, error) {
	switch input.MachineID {
	case 97:
		return models.QuoteCalculation{}, errors.New("database unavailable")
	case 98:
		return models.QuoteCalculation{}, &service.MissingConfigurationError{Message: "Falta la tarifa electrica"}
	case 99:
		return models.QuoteCalculation{}, service.ErrNotFound
	}
	if input.MaterialGrams == "0" {
		return models.QuoteCalculation{}, &service.ValidationError{Message: "Cantidad invalida"}
	}
	return models.QuoteCalculation{
		MachineID: input.MachineID, MaterialID: input.MaterialID, MaterialGrams: json.Number(input.MaterialGrams),
		DurationMinutes: input.DurationMinutes, PieceCount: input.PieceCount,
		MaterialCost: 4000, MachineCost: 5000, ElectricityCost: 175, PreparationCost: 1500,
		Subtotal: 10675, SuggestedPrice: 19675, SuggestedPricePerPiece: 9838,
	}, nil
}

func (fakeQuoteService) Create(_ context.Context, input service.CreateQuoteInput) (models.Quote, error) {
	if input.CustomerID == 99 {
		return models.Quote{}, &service.ValidationError{Message: "Cliente invalido"}
	}
	return models.Quote{ID: 7, CustomerID: input.CustomerID, StatusCode: "BORRADOR", TotalSuggestedPrice: 19675}, nil
}

func (fakeQuoteService) List(context.Context, service.QuoteFilters) ([]models.Quote, error) {
	return []models.Quote{{ID: 7, CustomerID: 3, StatusCode: "BORRADOR"}}, nil
}

func (fakeQuoteService) Get(_ context.Context, id int64) (models.Quote, error) {
	if id == 99 {
		return models.Quote{}, service.ErrNotFound
	}
	return models.Quote{ID: id, CustomerID: 3, StatusID: 1, StatusCode: "BORRADOR", Concepts: []models.QuoteConcept{}}, nil
}

func (fakeQuoteService) GeneratePDF(_ context.Context, id int64, input service.GenerateQuotePDFInput) (models.QuotePDF, error) {
	if id == 99 {
		return models.QuotePDF{}, service.ErrNotFound
	}
	if input.ValidityDays < 0 {
		return models.QuotePDF{}, &service.ValidationError{Message: "Vigencia invalida"}
	}
	return models.QuotePDF{Filename: "cotizacion-COT-000007.pdf", Content: []byte("%PDF-test")}, nil
}

func (fakeQuoteService) ListStatuses(context.Context) ([]models.QuoteStatus, error) {
	return []models.QuoteStatus{{ID: 1, Code: "BORRADOR", Name: "Borrador"}}, nil
}

func (fakeQuoteService) ChangeStatus(_ context.Context, id int64, status string) (models.Quote, error) {
	if status == "ACEPTADA" {
		return models.Quote{}, &service.ValidationError{Message: "Transicion invalida"}
	}
	return models.Quote{ID: id, StatusCode: status}, nil
}

func TestQuoteEndpoint(t *testing.T) {
	handler := New(Dependencies{Quotes: fakeQuoteService{}})
	tests := []struct {
		name     string
		body     string
		status   int
		contains string
	}{
		{"calculate", `{"maquina_id":1,"material_id":2,"cantidad_material_gramos":100,"duracion_minutos":120,"cantidad_piezas":2}`, http.StatusOK, `"precio_sugerido":196.75`},
		{"invalid input", `{"maquina_id":1,"material_id":2,"cantidad_material_gramos":0,"duracion_minutos":120,"cantidad_piezas":2}`, http.StatusUnprocessableEntity, `"code":"validation_failed"`},
		{"missing configuration", `{"maquina_id":98,"material_id":2,"cantidad_material_gramos":100,"duracion_minutos":120,"cantidad_piezas":2}`, http.StatusUnprocessableEntity, `"code":"missing_configuration"`},
		{"not found", `{"maquina_id":99,"material_id":2,"cantidad_material_gramos":100,"duracion_minutos":120,"cantidad_piezas":2}`, http.StatusNotFound, `"code":"not_found"`},
		{"internal error", `{"maquina_id":97,"material_id":2,"cantidad_material_gramos":100,"duracion_minutos":120,"cantidad_piezas":2}`, http.StatusInternalServerError, `"code":"internal_error"`},
		{"invalid json", `{`, http.StatusBadRequest, `"code":"invalid_json"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/cotizaciones/calcular", bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("body = %s, want it to contain %s", response.Body.String(), test.contains)
			}
		})
	}
}

func TestPersistentQuoteEndpoints(t *testing.T) {
	handler := New(Dependencies{Quotes: fakeQuoteService{}})
	tests := []struct {
		name, method, path, body string
		status                   int
		contains                 string
	}{
		{"create quote", http.MethodPost, "/api/cotizaciones", `{"cliente_id":3,"conceptos":[{"maquina_id":1,"material_id":2,"cantidad_material_gramos":100,"duracion_minutos":120,"cantidad_piezas":2}]}`, http.StatusCreated, `"estado_codigo":"BORRADOR"`},
		{"invalid customer", http.MethodPost, "/api/cotizaciones", `{"cliente_id":99,"conceptos":[]}`, http.StatusUnprocessableEntity, `"code":"validation_failed"`},
		{"list quotes", http.MethodGet, "/api/cotizaciones?cliente_id=3&estado=BORRADOR", "", http.StatusOK, `"id":7`},
		{"get quote", http.MethodGet, "/api/cotizaciones/7", "", http.StatusOK, `"estado_codigo":"BORRADOR"`},
		{"missing quote", http.MethodGet, "/api/cotizaciones/99", "", http.StatusNotFound, `"code":"not_found"`},
		{"generate PDF", http.MethodPost, "/api/cotizaciones/7/pdf", `{"vigencia_dias":15,"iva_porcentaje":16}`, http.StatusOK, "%PDF-test"},
		{"missing PDF quote", http.MethodPost, "/api/cotizaciones/99/pdf", `{}`, http.StatusNotFound, `"code":"not_found"`},
		{"list statuses", http.MethodGet, "/api/estados-cotizacion", "", http.StatusOK, `"codigo":"BORRADOR"`},
		{"change status", http.MethodPost, "/api/cotizaciones/7/cambios-estado", `{"estado_codigo":"ENVIADA"}`, http.StatusOK, `"estado_codigo":"ENVIADA"`},
		{"invalid transition", http.MethodPost, "/api/cotizaciones/7/cambios-estado", `{"estado_codigo":"ACEPTADA"}`, http.StatusUnprocessableEntity, `"code":"validation_failed"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("body = %s, want it to contain %s", response.Body.String(), test.contains)
			}
		})
	}
}

func TestQuotePDFResponseHeaders(t *testing.T) {
	handler := New(Dependencies{Quotes: fakeQuoteService{}})
	request := httptest.NewRequest(http.MethodPost, "/api/cotizaciones/7/pdf", bytes.NewBufferString(`{"vigencia_dias":15}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="cotizacion-COT-000007.pdf"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}
