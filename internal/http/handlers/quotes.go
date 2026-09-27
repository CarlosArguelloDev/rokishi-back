package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type QuoteService interface {
	Calculate(context.Context, service.CalculateQuoteInput) (models.QuoteCalculation, error)
}

type QuoteHandler struct {
	service QuoteService
}

func NewQuoteHandler(service QuoteService) *QuoteHandler {
	return &QuoteHandler{service: service}
}

type calculateQuoteRequest struct {
	MachineID       int64       `json:"maquina_id"`
	MaterialID      int64       `json:"material_id"`
	MaterialGrams   json.Number `json:"cantidad_material_gramos"`
	DurationMinutes int64       `json:"duracion_minutos"`
	PieceCount      int64       `json:"cantidad_piezas"`
}

func (h *QuoteHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	var request calculateQuoteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	calculation, err := h.service.Calculate(r.Context(), service.CalculateQuoteInput{
		MachineID: request.MachineID, MaterialID: request.MaterialID,
		MaterialGrams: request.MaterialGrams.String(), DurationMinutes: request.DurationMinutes,
		PieceCount: request.PieceCount,
	})
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.QuoteCalculation]{Data: calculation})
}

func writeQuoteError(w http.ResponseWriter, err error) {
	var validationError *service.ValidationError
	var configurationError *service.MissingConfigurationError
	switch {
	case errors.As(err, &validationError):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
	case errors.As(err, &configurationError):
		WriteError(w, http.StatusUnprocessableEntity, "missing_configuration", configurationError.Message)
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "La maquina o el material no existe")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}
