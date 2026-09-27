package handlers

import (
	"context"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type CustomerService interface {
	Create(context.Context, service.CreateCustomerInput) (models.Customer, error)
	List(context.Context, service.CustomerFilters) ([]models.Customer, error)
	Get(context.Context, int64) (models.Customer, error)
	Update(context.Context, int64, service.UpdateCustomerInput) (models.Customer, error)
}

type CustomerHandler struct {
	service CustomerService
}

func NewCustomerHandler(service CustomerService) *CustomerHandler {
	return &CustomerHandler{service: service}
}

type createCustomerRequest struct {
	Type  string  `json:"tipo"`
	Name  string  `json:"nombre"`
	Email *string `json:"correo"`
	Phone *string `json:"telefono"`
	Notes *string `json:"notas"`
}

type updateCustomerRequest struct {
	Type   optional[string] `json:"tipo"`
	Name   optional[string] `json:"nombre"`
	Email  optional[string] `json:"correo"`
	Phone  optional[string] `json:"telefono"`
	Notes  optional[string] `json:"notas"`
	Active optional[bool]   `json:"activo"`
}

func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createCustomerRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	customer, err := h.service.Create(r.Context(), service.CreateCustomerInput{
		Type: request.Type, Name: request.Name, Email: request.Email, Phone: request.Phone, Notes: request.Notes,
	})
	if err != nil {
		writeCatalogError(w, err, "customer_conflict", "El cliente ya existe", "Cliente no encontrado")
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Customer]{Data: customer})
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	active, ok := optionalBool(w, r, "activo")
	if !ok {
		return
	}
	var query *string
	if r.URL.Query().Has("q") {
		value := r.URL.Query().Get("q")
		query = &value
	}
	customers, err := h.service.List(r.Context(), service.CustomerFilters{Active: active, Query: query})
	if err != nil {
		writeCatalogError(w, err, "customer_conflict", "El cliente ya existe", "Cliente no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Customer]{Data: customers})
}

func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	customer, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "customer_conflict", "El cliente ya existe", "Cliente no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Customer]{Data: customer})
}

func (h *CustomerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request updateCustomerRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	customer, err := h.service.Update(r.Context(), id, service.UpdateCustomerInput{
		Type: toField(request.Type), Name: toField(request.Name), Email: toField(request.Email), Phone: toField(request.Phone),
		Notes: toField(request.Notes), Active: toField(request.Active),
	})
	if err != nil {
		writeCatalogError(w, err, "customer_conflict", "El cliente ya existe", "Cliente no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Customer]{Data: customer})
}
