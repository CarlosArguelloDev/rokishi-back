package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type ProductionService interface {
	CreateOrder(context.Context, int64) (models.Order, error)
	ListOrders(context.Context, service.OrderFilters) ([]models.Order, error)
	GetOrder(context.Context, int64) (models.Order, error)
	AssignMachine(context.Context, int64, int64) (models.Work, error)
	StartWork(context.Context, int64) (models.Work, error)
	FinishWork(context.Context, int64, service.FinishWorkInput) (models.Work, error)
}

type ProductionHandler struct {
	service ProductionService
}

func NewProductionHandler(service ProductionService) *ProductionHandler {
	return &ProductionHandler{service: service}
}

type assignMachineRequest struct {
	MachineID int64 `json:"maquina_id"`
}

type finishWorkRequest struct {
	Result           string      `json:"resultado"`
	ConsumedMaterial json.Number `json:"material_consumido_gramos"`
	Waste            json.Number `json:"desperdicio_gramos"`
	Notes            *string     `json:"notas"`
}

func (h *ProductionHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	quoteID, ok := pathID(w, r)
	if !ok {
		return
	}
	order, err := h.service.CreateOrder(r.Context(), quoteID)
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Order]{Data: order})
}

func (h *ProductionHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	customerID, ok := optionalPositiveID(w, r, "cliente_id")
	if !ok {
		return
	}
	var status *string
	if r.URL.Query().Has("estado") {
		value := r.URL.Query().Get("estado")
		status = &value
	}
	orders, err := h.service.ListOrders(r.Context(), service.OrderFilters{CustomerID: customerID, Status: status})
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Order]{Data: orders})
}

func (h *ProductionHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	order, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Order]{Data: order})
}

func (h *ProductionHandler) AssignMachine(w http.ResponseWriter, r *http.Request) {
	workID, ok := pathID(w, r)
	if !ok {
		return
	}
	var request assignMachineRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	work, err := h.service.AssignMachine(r.Context(), workID, request.MachineID)
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Work]{Data: work})
}

func (h *ProductionHandler) StartWork(w http.ResponseWriter, r *http.Request) {
	workID, ok := pathID(w, r)
	if !ok {
		return
	}
	work, err := h.service.StartWork(r.Context(), workID)
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Work]{Data: work})
}

func (h *ProductionHandler) FinishWork(w http.ResponseWriter, r *http.Request) {
	workID, ok := pathID(w, r)
	if !ok {
		return
	}
	var request finishWorkRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	work, err := h.service.FinishWork(r.Context(), workID, service.FinishWorkInput{
		Result: request.Result, ConsumedMaterial: request.ConsumedMaterial.String(),
		Waste: request.Waste.String(), Notes: request.Notes,
	})
	if err != nil {
		writeProductionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Work]{Data: work})
}

func writeProductionError(w http.ResponseWriter, err error) {
	var validationError *service.ValidationError
	switch {
	case errors.As(err, &validationError):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "El pedido, trabajo, cotizacion o maquina no existe")
	case errors.Is(err, service.ErrConflict):
		WriteError(w, http.StatusConflict, "order_exists", "La cotizacion ya tiene un pedido")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}
