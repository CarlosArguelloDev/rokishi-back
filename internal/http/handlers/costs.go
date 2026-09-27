package handlers

import (
	"context"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type MaterialService interface {
	Create(context.Context, service.CreateMaterialInput) (models.Material, error)
	List(context.Context, service.MaterialFilters) ([]models.Material, error)
	Get(context.Context, int64) (models.Material, error)
	Update(context.Context, int64, service.UpdateMaterialInput) (models.Material, error)
}

type RateService interface {
	GetMachineRate(context.Context, int64) (models.MachineRate, error)
	UpsertMachineRate(context.Context, int64, service.MachineRateInput) (models.MachineRate, error)
	GetEnergyRate(context.Context, int64) (models.EnergyRate, error)
	UpsertEnergyRate(context.Context, int64, service.EnergyRateInput) (models.EnergyRate, error)
}

type MaterialHandler struct {
	service MaterialService
}

func NewMaterialHandler(service MaterialService) *MaterialHandler {
	return &MaterialHandler{service: service}
}

type createMaterialRequest struct {
	Name      string  `json:"nombre"`
	Type      string  `json:"tipo"`
	Brand     *string `json:"marca"`
	Color     *string `json:"color"`
	CostPerKG float64 `json:"costo_por_kg"`
	StockKG   float64 `json:"stock_kg"`
}

type updateMaterialRequest struct {
	Name      optional[string]  `json:"nombre"`
	Type      optional[string]  `json:"tipo"`
	Brand     optional[string]  `json:"marca"`
	Color     optional[string]  `json:"color"`
	CostPerKG optional[float64] `json:"costo_por_kg"`
	StockKG   optional[float64] `json:"stock_kg"`
	Active    optional[bool]    `json:"activo"`
}

func (h *MaterialHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createMaterialRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	material, err := h.service.Create(r.Context(), service.CreateMaterialInput{
		Name: request.Name, Type: request.Type, Brand: request.Brand, Color: request.Color,
		CostPerKG: request.CostPerKG, StockKG: request.StockKG,
	})
	if err != nil {
		writeCatalogError(w, err, "material_conflict", "El material entra en conflicto con otro registro", "Material no encontrado")
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Material]{Data: material})
}

func (h *MaterialHandler) List(w http.ResponseWriter, r *http.Request) {
	filters := service.MaterialFilters{
		Type: optionalQueryValue(r, "tipo"), Brand: optionalQueryValue(r, "marca"), Color: optionalQueryValue(r, "color"),
	}
	active, ok := optionalBool(w, r, "activo")
	if !ok {
		return
	}
	filters.Active = active
	materials, err := h.service.List(r.Context(), filters)
	if err != nil {
		writeCatalogError(w, err, "material_conflict", "El material entra en conflicto con otro registro", "Material no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Material]{Data: materials})
}

func (h *MaterialHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	material, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "material_conflict", "El material entra en conflicto con otro registro", "Material no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Material]{Data: material})
}

func (h *MaterialHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request updateMaterialRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	material, err := h.service.Update(r.Context(), id, service.UpdateMaterialInput{
		Name: toField(request.Name), Type: toField(request.Type), Brand: toField(request.Brand),
		Color: toField(request.Color), CostPerKG: toField(request.CostPerKG),
		StockKG: toField(request.StockKG), Active: toField(request.Active),
	})
	if err != nil {
		writeCatalogError(w, err, "material_conflict", "El material entra en conflicto con otro registro", "Material no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Material]{Data: material})
}

type RateHandler struct {
	service RateService
}

func NewRateHandler(service RateService) *RateHandler {
	return &RateHandler{service: service}
}

type machineRateRequest struct {
	InternalCostHour float64 `json:"costo_interno_hora"`
	SalePriceHour    float64 `json:"precio_venta_hora"`
	PreparationCost  float64 `json:"costo_preparacion"`
}

type energyRateRequest struct {
	CostPerKWh float64 `json:"costo_por_kwh"`
}

func (h *RateHandler) GetMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rate, err := h.service.GetMachineRate(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "rate_conflict", "Conflicto al guardar la tarifa", "Tarifa o maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MachineRate]{Data: rate})
}

func (h *RateHandler) PutMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request machineRateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	rate, err := h.service.UpsertMachineRate(r.Context(), id, service.MachineRateInput{
		InternalCostHour: request.InternalCostHour, SalePriceHour: request.SalePriceHour, PreparationCost: request.PreparationCost,
	})
	if err != nil {
		writeCatalogError(w, err, "rate_conflict", "Conflicto al guardar la tarifa", "Maquina no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MachineRate]{Data: rate})
}

func (h *RateHandler) GetEnergy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rate, err := h.service.GetEnergyRate(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "rate_conflict", "Conflicto al guardar la tarifa", "Tarifa o locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.EnergyRate]{Data: rate})
}

func (h *RateHandler) PutEnergy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request energyRateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	rate, err := h.service.UpsertEnergyRate(r.Context(), id, service.EnergyRateInput{CostPerKWh: request.CostPerKWh})
	if err != nil {
		writeCatalogError(w, err, "rate_conflict", "Conflicto al guardar la tarifa", "Locacion no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.EnergyRate]{Data: rate})
}

func optionalQueryValue(r *http.Request, name string) *string {
	if !r.URL.Query().Has(name) {
		return nil
	}
	value := r.URL.Query().Get(name)
	return &value
}
