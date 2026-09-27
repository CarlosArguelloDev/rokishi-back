package handlers

import (
	"context"
	"net/http"
	"strconv"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type MachineService interface {
	Create(context.Context, service.CreateMachineInput) (models.Machine, error)
	List(context.Context, service.MachineFilters) ([]models.Machine, error)
	Get(context.Context, int64) (models.Machine, error)
	Update(context.Context, int64, service.UpdateMachineInput) (models.Machine, error)
}

type MachineHandler struct {
	service MachineService
}

func NewMachineHandler(service MachineService) *MachineHandler {
	return &MachineHandler{service: service}
}

type createMachineRequest struct {
	LocationID    int64    `json:"locacion_id"`
	MachineTypeID int64    `json:"tipo_maquina_id"`
	Code          string   `json:"codigo"`
	Name          string   `json:"nombre"`
	Brand         *string  `json:"marca"`
	Model         *string  `json:"modelo"`
	SerialNumber  *string  `json:"numero_serie"`
	PowerWatts    *float64 `json:"potencia_watts"`
	PurchaseDate  *string  `json:"fecha_compra"`
}

type updateMachineRequest struct {
	LocationID    optional[int64]   `json:"locacion_id"`
	MachineTypeID optional[int64]   `json:"tipo_maquina_id"`
	Code          optional[string]  `json:"codigo"`
	Name          optional[string]  `json:"nombre"`
	Brand         optional[string]  `json:"marca"`
	Model         optional[string]  `json:"modelo"`
	SerialNumber  optional[string]  `json:"numero_serie"`
	PowerWatts    optional[float64] `json:"potencia_watts"`
	PurchaseDate  optional[string]  `json:"fecha_compra"`
	Active        optional[bool]    `json:"activa"`
}

func (h *MachineHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createMachineRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	machine, err := h.service.Create(r.Context(), service.CreateMachineInput{
		LocationID: request.LocationID, MachineTypeID: request.MachineTypeID,
		Code: request.Code, Name: request.Name, Brand: request.Brand, Model: request.Model,
		SerialNumber: request.SerialNumber, PowerWatts: request.PowerWatts, PurchaseDate: request.PurchaseDate,
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una maquina con ese codigo", "Maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Machine]{Data: machine})
}

func (h *MachineHandler) List(w http.ResponseWriter, r *http.Request) {
	locationID, ok := optionalPositiveID(w, r, "locacion_id")
	if !ok {
		return
	}
	machineTypeID, ok := optionalPositiveID(w, r, "tipo_maquina_id")
	if !ok {
		return
	}
	active, ok := optionalBool(w, r, "activa")
	if !ok {
		return
	}
	var query *string
	if r.URL.Query().Has("q") {
		value := r.URL.Query().Get("q")
		query = &value
	}
	machines, err := h.service.List(r.Context(), service.MachineFilters{
		LocationID: locationID, MachineTypeID: machineTypeID, Active: active, Query: query,
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una maquina con ese codigo", "Maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Machine]{Data: machines})
}

func (h *MachineHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	machine, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una maquina con ese codigo", "Maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Machine]{Data: machine})
}

func (h *MachineHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request updateMachineRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	machine, err := h.service.Update(r.Context(), id, service.UpdateMachineInput{
		LocationID: toField(request.LocationID), MachineTypeID: toField(request.MachineTypeID),
		Code: toField(request.Code), Name: toField(request.Name), Brand: toField(request.Brand),
		Model: toField(request.Model), SerialNumber: toField(request.SerialNumber),
		PowerWatts: toField(request.PowerWatts), PurchaseDate: toField(request.PurchaseDate),
		Active: toField(request.Active),
	})
	if err != nil {
		writeCatalogError(w, err, "duplicate_code", "Ya existe una maquina con ese codigo", "Maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Machine]{Data: machine})
}

func optionalPositiveID(w http.ResponseWriter, r *http.Request, name string) (*int64, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		WriteError(w, http.StatusBadRequest, "invalid_filter", "El filtro "+name+" debe ser un entero positivo")
		return nil, false
	}
	return &id, true
}

func optionalBool(w http.ResponseWriter, r *http.Request, name string) (*bool, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_filter", "El filtro "+name+" debe ser true o false")
		return nil, false
	}
	return &parsed, true
}
