package service

import (
	"context"
	"math"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type MetricsFilters struct {
	From          *string
	To            *string
	MachineID     *int64
	LocationID    *int64
	MachineTypeID *int64
}

type metricsRepository interface {
	Summary(context.Context, repository.MetricsFilters) ([]repository.MachineMetricsData, error)
}

type MetricsService struct {
	repository metricsRepository
	now        func() time.Time
}

func NewMetricsService(repository metricsRepository) *MetricsService {
	return &MetricsService{repository: repository, now: time.Now}
}

func (s *MetricsService) Summary(ctx context.Context, filters MetricsFilters) (models.MetricsReport, error) {
	if err := validateMetricsIDs(filters); err != nil {
		return models.MetricsReport{}, err
	}
	now := s.now().UTC()
	from, err := parseOptionalTimestamp(filters.From, "desde")
	if err != nil {
		return models.MetricsReport{}, err
	}
	to, err := parseOptionalTimestamp(filters.To, "hasta")
	if err != nil {
		return models.MetricsReport{}, err
	}
	if to == nil || to.After(now) {
		to = &now
	}
	if from == nil {
		value := to.AddDate(0, 0, -30)
		from = &value
	}
	if !to.After(*from) {
		return models.MetricsReport{}, &ValidationError{Message: "El filtro hasta debe ser posterior a desde"}
	}

	data, err := s.repository.Summary(ctx, repository.MetricsFilters{
		From: from.UTC(), To: to.UTC(), MachineID: filters.MachineID,
		LocationID: filters.LocationID, MachineTypeID: filters.MachineTypeID,
	})
	if err != nil {
		return models.MetricsReport{}, err
	}

	report := models.MetricsReport{
		Period:   models.MetricsPeriod{From: from.UTC(), To: to.UTC()},
		Machines: make([]models.MachineMetrics, 0, len(data)),
		Limitations: []string{
			"Los periodos sin historial de estado no se infieren ni se contabilizan.",
			"Los ingresos y la utilidad usan los valores historicos de la cotizacion; los pedidos directos no tienen datos financieros.",
		},
	}
	var totals repository.MachineMetricsData
	for _, item := range data {
		report.Machines = append(report.Machines, machineMetrics(item))
		totals.RecordedSeconds += item.RecordedSeconds
		totals.WorkingSeconds += item.WorkingSeconds
		totals.AvailableSeconds += item.AvailableSeconds
		totals.OffSeconds += item.OffSeconds
		totals.MaintenanceSeconds += item.MaintenanceSeconds
		totals.FailureSeconds += item.FailureSeconds
		totals.PausedSeconds += item.PausedSeconds
		totals.CompletedJobs += item.CompletedJobs
		totals.FailedAttempts += item.FailedAttempts
		totals.ConsumedMaterialGrams += item.ConsumedMaterialGrams
		totals.WastedMaterialGrams += item.WastedMaterialGrams
		totals.EstimatedRevenueCents += item.EstimatedRevenueCents
		totals.EstimatedProfitCents += item.EstimatedProfitCents
		totals.JobsWithoutFinancialData += item.JobsWithoutFinancialData
	}
	report.Summary = metricValues(totals)
	return report, nil
}

func validateMetricsIDs(filters MetricsFilters) error {
	for name, value := range map[string]*int64{
		"maquina_id": filters.MachineID, "locacion_id": filters.LocationID,
		"tipo_maquina_id": filters.MachineTypeID,
	} {
		if value != nil && *value < 1 {
			return &ValidationError{Message: "El filtro " + name + " debe ser un entero positivo"}
		}
	}
	return nil
}

func machineMetrics(data repository.MachineMetricsData) models.MachineMetrics {
	return models.MachineMetrics{
		MachineID: data.MachineID, MachineCode: data.MachineCode,
		MachineName: data.MachineName, LocationID: data.LocationID,
		LocationName: data.LocationName, MachineTypeID: data.MachineTypeID,
		MachineType: data.MachineTypeName, MetricsValues: metricValues(data),
	}
}

func metricValues(data repository.MachineMetricsData) models.MetricsValues {
	finishedAttempts := data.CompletedJobs + data.FailedAttempts
	return models.MetricsValues{
		WorkingHours:          roundMetric(data.WorkingSeconds / 3600),
		AvailableHours:        roundMetric(data.AvailableSeconds / 3600),
		OffHours:              roundMetric(data.OffSeconds / 3600),
		MaintenanceHours:      roundMetric(data.MaintenanceSeconds / 3600),
		FailureHours:          roundMetric(data.FailureSeconds / 3600),
		PausedHours:           roundMetric(data.PausedSeconds / 3600),
		ProductiveHours:       roundMetric(data.WorkingSeconds / 3600),
		UnproductiveHours:     roundMetric((data.RecordedSeconds - data.WorkingSeconds) / 3600),
		RecordedHours:         roundMetric(data.RecordedSeconds / 3600),
		UtilizationPercentage: percentage(data.WorkingSeconds, data.RecordedSeconds),
		CompletedJobs:         data.CompletedJobs, FailedAttempts: data.FailedAttempts,
		SuccessRate:              percentage(float64(data.CompletedJobs), float64(finishedAttempts)),
		ConsumedMaterialGrams:    roundMetric(data.ConsumedMaterialGrams),
		WastedMaterialGrams:      roundMetric(data.WastedMaterialGrams),
		EstimatedRevenue:         models.Money(data.EstimatedRevenueCents),
		EstimatedProfit:          models.Money(data.EstimatedProfitCents),
		JobsWithoutFinancialData: data.JobsWithoutFinancialData,
	}
}

func percentage(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return roundMetric(numerator / denominator * 100)
}

func roundMetric(value float64) float64 {
	if value < 0 && value > -0.005 {
		return 0
	}
	return math.Round(value*100) / 100
}
