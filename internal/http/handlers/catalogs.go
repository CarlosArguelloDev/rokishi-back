package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type LocationService interface {
	Create(context.Context, service.CreateLocationInput) (models.Location, error)
	List(context.Context) ([]models.Location, error)
	Get(context.Context, int64) (models.Location, error)
	Update(context.Context, int64, service.UpdateLocationInput) (models.Location, error)
}

type MachineTypeService interface {
	Create(context.Context, service.CreateMachineTypeInput) (models.MachineType, error)
	List(context.Context) ([]models.MachineType, error)
	Get(context.Context, int64) (models.MachineType, error)
	Update(context.Context, int64, service.UpdateMachineTypeInput) (models.MachineType, error)
}

type LocationHandler struct {
	service LocationService
}

func NewLocationHandler(service LocationService) *LocationHandler {
	return &LocationHandler{service: service}
}

type createLocationRequest struct {
	Code     string  `json:"codigo"`
	Name     string  `json:"nombre"`
	Address  *string `json:"direccion"`
	City     *string `json:"ciudad"`
	State    *string `json:"estado"`
	Timezone *string `json:"zona_horaria"`
}

type updateLocationRequest struct {
	Code     optional[string] `json:"codigo"`
	Name     optional[string] `json:"nombre"`
	Address  optional[string] `json:"direccion"`
	City     optional[string] `json:"ciudad"`
	State    optional[string] `json:"estado"`
	Timezone optional[string] `json:"zona_horaria"`
	Active   optional[bool]   `json:"activa"`
}

func (h *LocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createLocationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	location, err := h.service.Create(r.Context(), service.CreateLocationInput{
		Code: request.Code, Name: request.Name, Address: request.Address, City: request.City,
		State: request.State, Timezone: request.Timezone,
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una locacion con ese codigo", "Locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Location]{Data: location})
}

func (h *LocationHandler) List(w http.ResponseWriter, r *http.Request) {
	locations, err := h.service.List(r.Context())
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una locacion con ese codigo", "Locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Location]{Data: locations})
}

func (h *LocationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	location, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una locacion con ese codigo", "Locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Location]{Data: location})
}

func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request updateLocationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	location, err := h.service.Update(r.Context(), id, service.UpdateLocationInput{
		Code: toField(request.Code), Name: toField(request.Name), Address: toField(request.Address),
		City: toField(request.City), State: toField(request.State), Timezone: toField(request.Timezone),
		Active: toField(request.Active),
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una locacion con ese codigo", "Locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Location]{Data: location})
}

type MachineTypeHandler struct {
	service MachineTypeService
}

func NewMachineTypeHandler(service MachineTypeService) *MachineTypeHandler {
	return &MachineTypeHandler{service: service}
}

type createMachineTypeRequest struct {
	Name        string  `json:"nombre"`
	Description *string `json:"descripcion"`
}

type updateMachineTypeRequest struct {
	Name        optional[string] `json:"nombre"`
	Description optional[string] `json:"descripcion"`
}

func (h *MachineTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createMachineTypeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	machineType, err := h.service.Create(r.Context(), service.CreateMachineTypeInput{
		Name: request.Name, Description: request.Description,
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_name", "Ya existe un tipo de maquina con ese nombre", "Tipo de maquina no encontrado")
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.MachineType]{Data: machineType})
}

func (h *MachineTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	machineTypes, err := h.service.List(r.Context())
	if err != nil {
		writeCatalogError(w, err, "duplicate_name", "Ya existe un tipo de maquina con ese nombre", "Tipo de maquina no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.MachineType]{Data: machineTypes})
}

func (h *MachineTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	machineType, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "duplicate_name", "Ya existe un tipo de maquina con ese nombre", "Tipo de maquina no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MachineType]{Data: machineType})
}

func (h *MachineTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request updateMachineTypeRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	machineType, err := h.service.Update(r.Context(), id, service.UpdateMachineTypeInput{
		Name: toField(request.Name), Description: toField(request.Description),
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_name", "Ya existe un tipo de maquina con ese nombre", "Tipo de maquina no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MachineType]{Data: machineType})
}

type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func toField[T any](value optional[T]) service.Field[T] {
	return service.Field[T]{Set: value.Set, Value: value.Value}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		WriteError(w, http.StatusBadRequest, "invalid_id", "El identificador debe ser un entero positivo")
		return 0, false
	}
	return id, true
}

func writeCatalogError(w http.ResponseWriter, err error, conflictCode, conflictMessage, notFoundMessage string) {
	var validationError *service.ValidationError
	switch {
	case errors.As(err, &validationError):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", notFoundMessage)
	case errors.Is(err, service.ErrConflict):
		WriteError(w, http.StatusConflict, conflictCode, conflictMessage)
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}
