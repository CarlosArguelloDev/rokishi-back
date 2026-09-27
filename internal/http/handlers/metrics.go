package handlers

import (
	"context"
	"errors"
	"net/http"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type MetricsService interface {
	Summary(context.Context, service.MetricsFilters) (models.MetricsReport, error)
}

type MetricsHandler struct {
	service MetricsService
}

func NewMetricsHandler(service MetricsService) *MetricsHandler {
	return &MetricsHandler{service: service}
}

func (h *MetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	machineID, ok := optionalPositiveID(w, r, "maquina_id")
	if !ok {
		return
	}
	locationID, ok := optionalPositiveID(w, r, "locacion_id")
	if !ok {
		return
	}
	machineTypeID, ok := optionalPositiveID(w, r, "tipo_maquina_id")
	if !ok {
		return
	}
	report, err := h.service.Summary(r.Context(), service.MetricsFilters{
		From: optionalMetricsValue(r, "desde"), To: optionalMetricsValue(r, "hasta"),
		MachineID: machineID, LocationID: locationID, MachineTypeID: machineTypeID,
	})
	if err != nil {
		var validationError *service.ValidationError
		if errors.As(err, &validationError) {
			WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.MetricsReport]{Data: report})
}

func optionalMetricsValue(r *http.Request, name string) *string {
	if !r.URL.Query().Has(name) {
		return nil
	}
	value := r.URL.Query().Get(name)
	return &value
}
