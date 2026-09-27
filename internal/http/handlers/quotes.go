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
	Create(context.Context, service.CreateQuoteInput) (models.Quote, error)
	List(context.Context, service.QuoteFilters) ([]models.Quote, error)
	Get(context.Context, int64) (models.Quote, error)
	ListStatuses(context.Context) ([]models.QuoteStatus, error)
	ChangeStatus(context.Context, int64, string) (models.Quote, error)
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

type createQuoteConceptRequest struct {
	Description     *string     `json:"descripcion"`
	MachineID       int64       `json:"maquina_id"`
	MaterialID      int64       `json:"material_id"`
	MaterialGrams   json.Number `json:"cantidad_material_gramos"`
	DurationMinutes int64       `json:"duracion_minutos"`
	PieceCount      int64       `json:"cantidad_piezas"`
}

type createQuoteRequest struct {
	CustomerID     int64                       `json:"cliente_id"`
	ExpirationDate *string                     `json:"fecha_vencimiento"`
	Notes          *string                     `json:"notas"`
	Concepts       []createQuoteConceptRequest `json:"conceptos"`
}

type changeQuoteStatusRequest struct {
	StatusCode string `json:"estado_codigo"`
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

func (h *QuoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createQuoteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	concepts := make([]service.CreateQuoteConceptInput, 0, len(request.Concepts))
	for _, concept := range request.Concepts {
		concepts = append(concepts, service.CreateQuoteConceptInput{
			Description: concept.Description, MachineID: concept.MachineID, MaterialID: concept.MaterialID,
			MaterialGrams: concept.MaterialGrams.String(), DurationMinutes: concept.DurationMinutes,
			PieceCount: concept.PieceCount,
		})
	}
	quote, err := h.service.Create(r.Context(), service.CreateQuoteInput{
		CustomerID: request.CustomerID, ExpirationDate: request.ExpirationDate,
		Notes: request.Notes, Concepts: concepts,
	})
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.Quote]{Data: quote})
}

func (h *QuoteHandler) List(w http.ResponseWriter, r *http.Request) {
	customerID, ok := optionalPositiveID(w, r, "cliente_id")
	if !ok {
		return
	}
	var statusCode *string
	if r.URL.Query().Has("estado") {
		value := r.URL.Query().Get("estado")
		statusCode = &value
	}
	quotes, err := h.service.List(r.Context(), service.QuoteFilters{
		CustomerID: customerID, StatusCode: statusCode,
	})
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.Quote]{Data: quotes})
}

func (h *QuoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	quote, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Quote]{Data: quote})
}

func (h *QuoteHandler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.service.ListStatuses(r.Context())
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.QuoteStatus]{Data: statuses})
}

func (h *QuoteHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var request changeQuoteStatusRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	quote, err := h.service.ChangeStatus(r.Context(), id, request.StatusCode)
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.Quote]{Data: quote})
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
		WriteError(w, http.StatusNotFound, "not_found", "La cotizacion, maquina o material no existe")
	case errors.Is(err, service.ErrConflict):
		WriteError(w, http.StatusConflict, "quote_state_conflict", "La cotizacion cambio de estado; vuelve a consultarla")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}
