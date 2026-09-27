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

type fakeCustomerService struct{}

func (fakeCustomerService) Create(_ context.Context, input service.CreateCustomerInput) (models.Customer, error) {
	if input.Name == "" {
		return models.Customer{}, &service.ValidationError{Message: "Nombre obligatorio"}
	}
	return models.Customer{ID: 1, Name: input.Name, Active: true}, nil
}

func (fakeCustomerService) List(context.Context, service.CustomerFilters) ([]models.Customer, error) {
	return []models.Customer{{ID: 1, Name: "Taller Norte", Active: true}}, nil
}

func (fakeCustomerService) Get(_ context.Context, id int64) (models.Customer, error) {
	if id == 99 {
		return models.Customer{}, service.ErrNotFound
	}
	return models.Customer{ID: id, Name: "Taller Norte", Active: true}, nil
}

func (fakeCustomerService) Update(_ context.Context, id int64, input service.UpdateCustomerInput) (models.Customer, error) {
	active := true
	if input.Active.Set && input.Active.Value != nil {
		active = *input.Active.Value
	}
	return models.Customer{ID: id, Name: "Taller Norte", Active: active}, nil
}

func TestCustomerEndpoints(t *testing.T) {
	handler := New(Dependencies{Customers: fakeCustomerService{}})
	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{"create", http.MethodPost, "/api/clientes", `{"nombre":"Taller Norte","correo":"ventas@example.com"}`, http.StatusCreated},
		{"invalid", http.MethodPost, "/api/clientes", `{}`, http.StatusUnprocessableEntity},
		{"list", http.MethodGet, "/api/clientes?activo=true&q=norte", "", http.StatusOK},
		{"get", http.MethodGet, "/api/clientes/1", "", http.StatusOK},
		{"deactivate", http.MethodPatch, "/api/clientes/1", `{"activo":false}`, http.StatusOK},
		{"missing", http.MethodGet, "/api/clientes/99", "", http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}
