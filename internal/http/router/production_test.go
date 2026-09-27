package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeProductionService struct{}

func (fakeProductionService) CreateOrder(_ context.Context, quoteID int64) (models.Order, error) {
	if quoteID == 99 {
		return models.Order{}, service.ErrConflict
	}
	return models.Order{ID: 4, QuoteID: quoteID, Status: "PENDIENTE", WorkCount: 1}, nil
}

func (fakeProductionService) ListOrders(context.Context, service.OrderFilters) ([]models.Order, error) {
	return []models.Order{{ID: 4, QuoteID: 7, Status: "PENDIENTE"}}, nil
}

func (fakeProductionService) GetOrder(_ context.Context, id int64) (models.Order, error) {
	return models.Order{ID: id, QuoteID: 7, Status: "EN_PRODUCCION", Works: []models.Work{}}, nil
}

func (fakeProductionService) AssignMachine(_ context.Context, workID, machineID int64) (models.Work, error) {
	return models.Work{ID: workID, MachineID: &machineID, Status: "PENDIENTE"}, nil
}

func (fakeProductionService) StartWork(_ context.Context, workID int64) (models.Work, error) {
	return models.Work{ID: workID, Status: "EN_PROCESO"}, nil
}

func (fakeProductionService) FinishWork(_ context.Context, workID int64, input service.FinishWorkInput) (models.Work, error) {
	if input.Result == "INVALIDO" {
		return models.Work{}, &service.ValidationError{Message: "Resultado invalido"}
	}
	return models.Work{ID: workID, Status: "COMPLETADO"}, nil
}

func TestProductionEndpoints(t *testing.T) {
	handler := New(Dependencies{Production: fakeProductionService{}})
	tests := []struct {
		name, method, path, body string
		status                   int
		contains                 string
	}{
		{"create order", http.MethodPost, "/api/cotizaciones/7/pedido", "", http.StatusCreated, `"cotizacion_id":7`},
		{"duplicate order", http.MethodPost, "/api/cotizaciones/99/pedido", "", http.StatusConflict, `"code":"order_exists"`},
		{"list orders", http.MethodGet, "/api/pedidos?estado=PENDIENTE", "", http.StatusOK, `"id":4`},
		{"get order", http.MethodGet, "/api/pedidos/4", "", http.StatusOK, `"estado":"EN_PRODUCCION"`},
		{"assign machine", http.MethodPatch, "/api/trabajos/9/asignacion", `{"maquina_id":6}`, http.StatusOK, `"maquina_id":6`},
		{"start work", http.MethodPost, "/api/trabajos/9/iniciar", "", http.StatusCreated, `"estado":"EN_PROCESO"`},
		{"finish work", http.MethodPost, "/api/trabajos/9/finalizar", `{"resultado":"EXITOSO","material_consumido_gramos":95.5,"desperdicio_gramos":2}`, http.StatusOK, `"estado":"COMPLETADO"`},
		{"invalid finish", http.MethodPost, "/api/trabajos/9/finalizar", `{"resultado":"INVALIDO","material_consumido_gramos":1,"desperdicio_gramos":0}`, http.StatusUnprocessableEntity, `"code":"validation_failed"`},
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
