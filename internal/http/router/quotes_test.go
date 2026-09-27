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
