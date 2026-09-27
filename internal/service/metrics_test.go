package service

import (
	"context"
	"testing"
	"time"

	"rokishi-back/internal/repository"
)

type fakeMetricsRepository struct {
	filters repository.MetricsFilters
	data    []repository.MachineMetricsData
	err     error
}

func (f *fakeMetricsRepository) Summary(_ context.Context, filters repository.MetricsFilters) ([]repository.MachineMetricsData, error) {
	f.filters = filters
	return f.data, f.err
}

func TestMetricsServiceCalculatesSummary(t *testing.T) {
	repository := &fakeMetricsRepository{data: []repository.MachineMetricsData{
		{
			MachineID: 1, MachineCode: "FDM-01", MachineName: "A1", LocationID: 2,
			LocationName: "Taller", MachineTypeID: 3, MachineTypeName: "FDM",
			RecordedSeconds: 36000, WorkingSeconds: 14400, AvailableSeconds: 10800,
			OffSeconds: 3600, MaintenanceSeconds: 3600, FailureSeconds: 3600,
			CompletedJobs: 3, FailedAttempts: 1, ConsumedMaterialGrams: 250.125,
			WastedMaterialGrams: 12.5, EstimatedRevenueCents: 150000,
			EstimatedProfitCents: 50000, JobsWithoutFinancialData: 1,
		},
		{MachineID: 2, RecordedSeconds: 18000, WorkingSeconds: 3600, CompletedJobs: 1, FailedAttempts: 1},
	}}
	metrics := NewMetricsService(repository)
	now := time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)
	metrics.now = func() time.Time { return now }

	report, err := metrics.Summary(context.Background(), MetricsFilters{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Period.From.Equal(now.AddDate(0, 0, -30)) || !report.Period.To.Equal(now) {
		t.Fatalf("unexpected period: %+v", report.Period)
	}
	if report.Summary.RecordedHours != 15 || report.Summary.WorkingHours != 5 || report.Summary.UtilizationPercentage != 33.33 {
		t.Fatalf("unexpected operational summary: %+v", report.Summary)
	}
	if report.Summary.CompletedJobs != 4 || report.Summary.FailedAttempts != 2 || report.Summary.SuccessRate != 66.67 {
		t.Fatalf("unexpected production summary: %+v", report.Summary)
	}
	if report.Summary.EstimatedRevenue != 150000 || report.Summary.EstimatedProfit != 50000 {
		t.Fatalf("unexpected financial summary: %+v", report.Summary)
	}
	if len(report.Machines) != 2 || report.Machines[0].UtilizationPercentage != 40 {
		t.Fatalf("unexpected machine metrics: %+v", report.Machines)
	}
}

func TestMetricsServiceValidatesRangeAndFilters(t *testing.T) {
	metrics := NewMetricsService(&fakeMetricsRepository{})
	metrics.now = func() time.Time { return time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC) }
	invalid := "not-a-date"
	if _, err := metrics.Summary(context.Background(), MetricsFilters{From: &invalid}); !isValidationError(err) {
		t.Fatalf("expected invalid date error, got %v", err)
	}
	from := "2026-09-20T00:00:00Z"
	to := "2026-09-19T00:00:00Z"
	if _, err := metrics.Summary(context.Background(), MetricsFilters{From: &from, To: &to}); !isValidationError(err) {
		t.Fatalf("expected invalid range error, got %v", err)
	}
	invalidID := int64(0)
	if _, err := metrics.Summary(context.Background(), MetricsFilters{MachineID: &invalidID}); !isValidationError(err) {
		t.Fatalf("expected invalid filter error, got %v", err)
	}
}
