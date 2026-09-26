package router

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeLocationService struct {
	create func(service.CreateLocationInput) (models.Location, error)
	list   func() ([]models.Location, error)
	get    func(int64) (models.Location, error)
	update func(int64, service.UpdateLocationInput) (models.Location, error)
}

func (f fakeLocationService) Create(_ context.Context, input service.CreateLocationInput) (models.Location, error) {
	return f.create(input)
}

func (f fakeLocationService) List(context.Context) ([]models.Location, error) {
	return f.list()
}

func (f fakeLocationService) Get(_ context.Context, id int64) (models.Location, error) {
	return f.get(id)
}

func (f fakeLocationService) Update(_ context.Context, id int64, input service.UpdateLocationInput) (models.Location, error) {
	return f.update(id, input)
}

type fakeMachineTypeService struct {
	create func(service.CreateMachineTypeInput) (models.MachineType, error)
	list   func() ([]models.MachineType, error)
	get    func(int64) (models.MachineType, error)
	update func(int64, service.UpdateMachineTypeInput) (models.MachineType, error)
}

func (f fakeMachineTypeService) Create(_ context.Context, input service.CreateMachineTypeInput) (models.MachineType, error) {
	return f.create(input)
}

func (f fakeMachineTypeService) List(context.Context) ([]models.MachineType, error) {
	return f.list()
}

func (f fakeMachineTypeService) Get(_ context.Context, id int64) (models.MachineType, error) {
	return f.get(id)
}

func (f fakeMachineTypeService) Update(_ context.Context, id int64, input service.UpdateMachineTypeInput) (models.MachineType, error) {
	return f.update(id, input)
}

func TestLocationEndpoints(t *testing.T) {
	location := models.Location{ID: 1, Code: "MX-01", Name: "Centro", Timezone: "America/Mexico_City", Active: true}
	locationService := fakeLocationService{
		create: func(input service.CreateLocationInput) (models.Location, error) {
			if input.Code == "DUP" {
				return models.Location{}, service.ErrConflict
			}
			if input.Name == "" {
				return models.Location{}, &service.ValidationError{Message: "El campo nombre es obligatorio"}
			}
			return location, nil
		},
		list: func() ([]models.Location, error) { return []models.Location{location}, nil },
		get: func(id int64) (models.Location, error) {
			if id == 99 {
				return models.Location{}, service.ErrNotFound
			}
			return location, nil
		},
		update: func(_ int64, input service.UpdateLocationInput) (models.Location, error) {
			if input.Active.Set && input.Active.Value != nil {
				location.Active = *input.Active.Value
			}
			return location, nil
		},
	}
	handler := New(Dependencies{Ping: func(context.Context) error { return nil }, Locations: locationService})

	for _, tc := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"create", http.MethodPost, "/api/locaciones", `{"codigo":"MX-01","nombre":"Centro"}`, http.StatusCreated},
		{"list", http.MethodGet, "/api/locaciones", "", http.StatusOK},
		{"get", http.MethodGet, "/api/locaciones/1", "", http.StatusOK},
		{"update active", http.MethodPatch, "/api/locaciones/1", `{"activa":false}`, http.StatusOK},
		{"missing field", http.MethodPost, "/api/locaciones", `{"codigo":"MX-01"}`, http.StatusUnprocessableEntity},
		{"duplicate", http.MethodPost, "/api/locaciones", `{"codigo":"DUP","nombre":"Centro"}`, http.StatusConflict},
		{"not found", http.MethodGet, "/api/locaciones/99", "", http.StatusNotFound},
		{"invalid id", http.MethodGet, "/api/locaciones/no", "", http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/api/locaciones", `{"codigo":"MX","nombre":"Centro","otro":true}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tc.status, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Fatalf("content type = %q", got)
			}
		})
	}
}

func TestMachineTypeEndpoints(t *testing.T) {
	machineType := models.MachineType{ID: 1, Name: "FDM"}
	machineTypeService := fakeMachineTypeService{
		create: func(service.CreateMachineTypeInput) (models.MachineType, error) { return machineType, nil },
		list:   func() ([]models.MachineType, error) { return []models.MachineType{machineType}, nil },
		get: func(id int64) (models.MachineType, error) {
			if id == 99 {
				return models.MachineType{}, service.ErrNotFound
			}
			return machineType, nil
		},
		update: func(_ int64, input service.UpdateMachineTypeInput) (models.MachineType, error) {
			if input.Name.Set && input.Name.Value != nil {
				machineType.Name = *input.Name.Value
			}
			return machineType, nil
		},
	}
	handler := New(Dependencies{Ping: func(context.Context) error { return nil }, MachineTypes: machineTypeService})

	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/api/tipos-maquina", `{"nombre":"FDM"}`, http.StatusCreated},
		{http.MethodGet, "/api/tipos-maquina", "", http.StatusOK},
		{http.MethodGet, "/api/tipos-maquina/1", "", http.StatusOK},
		{http.MethodPatch, "/api/tipos-maquina/1", `{"nombre":"Resina"}`, http.StatusOK},
		{http.MethodGet, "/api/tipos-maquina/99", "", http.StatusNotFound},
	} {
		request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s %s: status = %d, want %d; body = %s", tc.method, tc.path, response.Code, tc.status, response.Body.String())
		}
	}
}

func TestCatalogInternalErrorIsHidden(t *testing.T) {
	locationService := fakeLocationService{
		list: func() ([]models.Location, error) { return nil, errors.New("password=secret") },
	}
	handler := New(Dependencies{Ping: func(context.Context) error { return nil }, Locations: locationService})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/locaciones", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("secret")) {
		t.Fatalf("internal error leaked: %s", response.Body.String())
	}
}
