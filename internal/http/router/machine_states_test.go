package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeMachineStateService struct{}

func (fakeMachineStateService) List(context.Context) ([]models.MachineState, error) {
	return []models.MachineState{{ID: 1, Code: "DISPONIBLE", Name: "Disponible"}}, nil
}

func (fakeMachineStateService) Current(_ context.Context, machineID int64) (models.MachineStatePeriod, error) {
	if machineID == 99 {
		return models.MachineStatePeriod{}, service.ErrNotFound
	}
	return models.MachineStatePeriod{ID: 1, MachineID: machineID, StateID: 1, StateCode: "DISPONIBLE", StateName: "Disponible", StartedAt: time.Now()}, nil
}

func (fakeMachineStateService) History(_ context.Context, machineID int64, filters service.StateHistoryFilters) ([]models.MachineStatePeriod, error) {
	if filters.From != nil && *filters.From == "invalid" {
		return nil, &service.ValidationError{Message: "El campo desde debe tener formato RFC3339"}
	}
	return []models.MachineStatePeriod{{ID: 1, MachineID: machineID}}, nil
}

func (fakeMachineStateService) Change(_ context.Context, machineID int64, input service.ChangeMachineStateInput) (models.MachineStateChange, error) {
	if input.StateID == 1 {
		return models.MachineStateChange{}, service.ErrStateUnchanged
	}
	return models.MachineStateChange{Current: models.MachineStatePeriod{ID: 2, MachineID: machineID, StateID: input.StateID}}, nil
}

func TestMachineStateEndpoints(t *testing.T) {
	handler := New(Dependencies{
		Ping:          func(context.Context) error { return nil },
		MachineStates: fakeMachineStateService{},
	})
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"list states", http.MethodGet, "/api/estados-maquina", "", http.StatusOK},
		{"current", http.MethodGet, "/api/maquinas/2/estado-actual", "", http.StatusOK},
		{"change", http.MethodPost, "/api/maquinas/2/cambios-estado", `{"estado_maquina_id":2,"notas":"Inicio"}`, http.StatusCreated},
		{"history", http.MethodGet, "/api/maquinas/2/historial-estados?desde=2026-09-01T00:00:00Z", "", http.StatusOK},
		{"missing current", http.MethodGet, "/api/maquinas/99/estado-actual", "", http.StatusNotFound},
		{"unchanged", http.MethodPost, "/api/maquinas/2/cambios-estado", `{"estado_maquina_id":1}`, http.StatusConflict},
		{"invalid history", http.MethodGet, "/api/maquinas/2/historial-estados?desde=invalid", "", http.StatusUnprocessableEntity},
		{"invalid json", http.MethodPost, "/api/maquinas/2/cambios-estado", `{`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
}
