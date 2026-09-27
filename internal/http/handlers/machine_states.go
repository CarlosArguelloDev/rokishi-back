package handlers

import (
	"context"
	"errors"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type MachineStateService interface {
	List(context.Context) ([]models.MachineState, error)
	Current(context.Context, int64) (models.MachineStatePeriod, error)
	History(context.Context, int64, service.StateHistoryFilters) ([]models.MachineStatePeriod, error)
	Change(context.Context, int64, service.ChangeMachineStateInput) (models.MachineStateChange, error)
}

type MachineStateHandler struct {
	service MachineStateService
}

func NewMachineStateHandler(service MachineStateService) *MachineStateHandler {
	return &MachineStateHandler{service: service}
}

type changeMachineStateRequest struct {
	StateID   int64   `json:"estado_maquina_id"`
	StartedAt *string `json:"fecha_inicio"`
	Notes     *string `json:"notas"`
}

func (h *MachineStateHandler) List(w http.ResponseWriter, r *http.Request) {
	states, err := h.service.List(r.Context())
	if err != nil {
		writeMachineStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.MachineState]{Data: states})
}

func (h *MachineStateHandler) Current(w http.ResponseWriter, r *http.Request) {
	machineID, ok := pathID(w, r)
	if !ok {
		return
	}
	period, err := h.service.Current(r.Context(), machineID)
	if err != nil {
		writeMachineStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MachineStatePeriod]{Data: period})
}

func (h *MachineStateHandler) Change(w http.ResponseWriter, r *http.Request) {
	machineID, ok := pathID(w, r)
	if !ok {
		return
	}
	var request changeMachineStateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	change, err := h.service.Change(r.Context(), machineID, service.ChangeMachineStateInput{
		StateID: request.StateID, StartedAt: request.StartedAt, Notes: request.Notes,
	})
	if err != nil {
		writeMachineStateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.MachineStateChange]{Data: change})
}

func (h *MachineStateHandler) History(w http.ResponseWriter, r *http.Request) {
	machineID, ok := pathID(w, r)
	if !ok {
		return
	}
	var from, to *string
	if r.URL.Query().Has("desde") {
		value := r.URL.Query().Get("desde")
		from = &value
	}
	if r.URL.Query().Has("hasta") {
		value := r.URL.Query().Get("hasta")
		to = &value
	}
	history, err := h.service.History(r.Context(), machineID, service.StateHistoryFilters{From: from, To: to})
	if err != nil {
		writeMachineStateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.MachineStatePeriod]{Data: history})
}

func writeMachineStateError(w http.ResponseWriter, err error) {
	var validationError *service.ValidationError
	switch {
	case errors.As(err, &validationError):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
	case errors.Is(err, service.ErrStateUnchanged):
		WriteError(w, http.StatusConflict, "state_unchanged", "La maquina ya tiene ese estado")
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "La maquina, el estado o el periodo actual no existe")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}
