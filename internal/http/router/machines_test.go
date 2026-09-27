package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeMachineService struct {
	create func(service.CreateMachineInput) (models.Machine, error)
	list   func(service.MachineFilters) ([]models.Machine, error)
	get    func(int64) (models.Machine, error)
	update func(int64, service.UpdateMachineInput) (models.Machine, error)
}

func (f fakeMachineService) Create(_ context.Context, input service.CreateMachineInput) (models.Machine, error) {
	return f.create(input)
}

func (f fakeMachineService) List(_ context.Context, filters service.MachineFilters) ([]models.Machine, error) {
	return f.list(filters)
}

func (f fakeMachineService) Get(_ context.Context, id int64) (models.Machine, error) {
	return f.get(id)
}

func (f fakeMachineService) Update(_ context.Context, id int64, input service.UpdateMachineInput) (models.Machine, error) {
	return f.update(id, input)
}

func TestMachineEndpoints(t *testing.T) {
	machine := models.Machine{ID: 1, LocationID: 1, MachineTypeID: 2, Code: "PR-01", Name: "MK4", Active: true}
	machineService := fakeMachineService{
		create: func(input service.CreateMachineInput) (models.Machine, error) {
			if input.Code == "DUP" {
				return models.Machine{}, service.ErrConflict
			}
			if input.LocationID == 99 {
				return models.Machine{}, &service.ValidationError{Message: "La locacion indicada no existe"}
			}
			return machine, nil
		},
		list: func(filters service.MachineFilters) ([]models.Machine, error) {
			if filters.LocationID != nil && *filters.LocationID != 1 {
				t.Fatalf("unexpected location filter: %d", *filters.LocationID)
			}
			return []models.Machine{machine}, nil
		},
		get: func(id int64) (models.Machine, error) {
			if id == 99 {
				return models.Machine{}, service.ErrNotFound
			}
			return machine, nil
		},
		update: func(_ int64, input service.UpdateMachineInput) (models.Machine, error) {
			if input.Active.Set && input.Active.Value != nil {
				machine.Active = *input.Active.Value
			}
			return machine, nil
		},
	}
	handler := New(Dependencies{Ping: func(context.Context) error { return nil }, Machines: machineService})

	for _, tc := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"create", http.MethodPost, "/api/maquinas", `{"locacion_id":1,"tipo_maquina_id":2,"codigo":"PR-01","nombre":"MK4"}`, http.StatusCreated},
		{"list with filters", http.MethodGet, "/api/maquinas?locacion_id=1&tipo_maquina_id=2&activa=true&q=mk", "", http.StatusOK},
		{"get", http.MethodGet, "/api/maquinas/1", "", http.StatusOK},
		{"deactivate", http.MethodPatch, "/api/maquinas/1", `{"activa":false}`, http.StatusOK},
		{"duplicate", http.MethodPost, "/api/maquinas", `{"locacion_id":1,"tipo_maquina_id":2,"codigo":"DUP","nombre":"MK4"}`, http.StatusConflict},
		{"missing location", http.MethodPost, "/api/maquinas", `{"locacion_id":99,"tipo_maquina_id":2,"codigo":"M-2","nombre":"MK4"}`, http.StatusUnprocessableEntity},
		{"not found", http.MethodGet, "/api/maquinas/99", "", http.StatusNotFound},
		{"invalid filter", http.MethodGet, "/api/maquinas?activa=quizas", "", http.StatusBadRequest},
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
