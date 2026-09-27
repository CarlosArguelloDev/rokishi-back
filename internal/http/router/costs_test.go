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

type fakeMaterialService struct{}

func (fakeMaterialService) Create(_ context.Context, input service.CreateMaterialInput) (models.Material, error) {
	if input.CostPerKG < 0 {
		return models.Material{}, &service.ValidationError{Message: "Costo invalido"}
	}
	return models.Material{ID: 1, Name: input.Name, Type: input.Type, Active: true}, nil
}

func (fakeMaterialService) List(_ context.Context, filters service.MaterialFilters) ([]models.Material, error) {
	return []models.Material{{ID: 1, Name: "PLA", Type: "Filamento", Active: true}}, nil
}

func (fakeMaterialService) Get(_ context.Context, id int64) (models.Material, error) {
	if id == 99 {
		return models.Material{}, service.ErrNotFound
	}
	return models.Material{ID: id, Name: "PLA", Type: "Filamento", Active: true}, nil
}

func (fakeMaterialService) Update(_ context.Context, id int64, input service.UpdateMaterialInput) (models.Material, error) {
	active := true
	if input.Active.Set && input.Active.Value != nil {
		active = *input.Active.Value
	}
	return models.Material{ID: id, Name: "PLA", Type: "Filamento", Active: active}, nil
}

type fakeRateService struct{}

func (fakeRateService) GetMachineRate(_ context.Context, machineID int64) (models.MachineRate, error) {
	if machineID == 99 {
		return models.MachineRate{}, service.ErrNotFound
	}
	return models.MachineRate{ID: 1, MachineID: machineID}, nil
}

func (fakeRateService) UpsertMachineRate(_ context.Context, machineID int64, input service.MachineRateInput) (models.MachineRate, error) {
	if input.InternalCostHour < 0 {
		return models.MachineRate{}, &service.ValidationError{Message: "Costo invalido"}
	}
	return models.MachineRate{ID: 1, MachineID: machineID, InternalCostHour: input.InternalCostHour}, nil
}

func (fakeRateService) GetEnergyRate(_ context.Context, locationID int64) (models.EnergyRate, error) {
	return models.EnergyRate{ID: 1, LocationID: locationID}, nil
}

func (fakeRateService) UpsertEnergyRate(_ context.Context, locationID int64, input service.EnergyRateInput) (models.EnergyRate, error) {
	return models.EnergyRate{ID: 1, LocationID: locationID, CostPerKWh: input.CostPerKWh}, nil
}

func TestMaterialAndRateEndpoints(t *testing.T) {
	handler := New(Dependencies{
		Ping:      func(context.Context) error { return nil },
		Materials: fakeMaterialService{},
		Rates:     fakeRateService{},
	})
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"create material", http.MethodPost, "/api/materiales", `{"nombre":"PLA","tipo":"Filamento","costo_por_kg":400,"stock_kg":2}`, http.StatusCreated},
		{"list materials", http.MethodGet, "/api/materiales?tipo=Filamento&activo=true", "", http.StatusOK},
		{"get material", http.MethodGet, "/api/materiales/1", "", http.StatusOK},
		{"deactivate material", http.MethodPatch, "/api/materiales/1", `{"activo":false}`, http.StatusOK},
		{"invalid material", http.MethodPost, "/api/materiales", `{"nombre":"PLA","tipo":"Filamento","costo_por_kg":-1}`, http.StatusUnprocessableEntity},
		{"missing material", http.MethodGet, "/api/materiales/99", "", http.StatusNotFound},
		{"get machine rate", http.MethodGet, "/api/maquinas/1/tarifa", "", http.StatusOK},
		{"put machine rate", http.MethodPut, "/api/maquinas/1/tarifa", `{"costo_interno_hora":20,"precio_venta_hora":60,"costo_preparacion":10}`, http.StatusOK},
		{"invalid machine rate", http.MethodPut, "/api/maquinas/1/tarifa", `{"costo_interno_hora":-1}`, http.StatusUnprocessableEntity},
		{"missing machine rate", http.MethodGet, "/api/maquinas/99/tarifa", "", http.StatusNotFound},
		{"get energy rate", http.MethodGet, "/api/locaciones/1/tarifa-energia", "", http.StatusOK},
		{"put energy rate", http.MethodPut, "/api/locaciones/1/tarifa-energia", `{"costo_por_kwh":2.3456}`, http.StatusOK},
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
