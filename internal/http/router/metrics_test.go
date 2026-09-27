package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeMetricsService struct{}

func (fakeMetricsService) Summary(_ context.Context, filters service.MetricsFilters) (models.MetricsReport, error) {
	if filters.From != nil && *filters.From == "invalid" {
		return models.MetricsReport{}, &service.ValidationError{Message: "Fecha invalida"}
	}
	return models.MetricsReport{
		Period: models.MetricsPeriod{
			From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
		Summary:  models.MetricsValues{WorkingHours: 12.5, CompletedJobs: 4},
		Machines: []models.MachineMetrics{},
	}, nil
}

func TestMetricsEndpoint(t *testing.T) {
	handler := New(Dependencies{Metrics: fakeMetricsService{}})
	tests := []struct {
		name   string
		path   string
		status int
		text   string
	}{
		{"summary", "/api/metricas/resumen?desde=2026-09-01T00:00:00Z&maquina_id=2", http.StatusOK, `"horas_trabajando":12.5`},
		{"invalid date", "/api/metricas/resumen?desde=invalid", http.StatusUnprocessableEntity, `"code":"validation_failed"`},
		{"invalid machine", "/api/metricas/resumen?maquina_id=0", http.StatusBadRequest, `"code":"invalid_filter"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.text) {
				t.Fatalf("body = %s, want %s", response.Body.String(), test.text)
			}
		})
	}
}
