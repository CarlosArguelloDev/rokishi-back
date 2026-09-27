package service

import (
	"context"
	"errors"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeMaterialRepository struct {
	material models.Material
	filters  repository.MaterialFilters
	err      error
}

func (f *fakeMaterialRepository) Create(_ context.Context, material models.Material) (models.Material, error) {
	material.ID = 1
	f.material = material
	return material, f.err
}

func (f *fakeMaterialRepository) List(_ context.Context, filters repository.MaterialFilters) ([]models.Material, error) {
	f.filters = filters
	return []models.Material{f.material}, f.err
}

func (f *fakeMaterialRepository) Get(context.Context, int64) (models.Material, error) {
	return f.material, f.err
}

func (f *fakeMaterialRepository) Update(_ context.Context, material models.Material) (models.Material, error) {
	f.material = material
	return material, f.err
}

func TestMaterialServiceCreateUpdateAndFilters(t *testing.T) {
	repository := &fakeMaterialRepository{}
	service := NewMaterialService(repository)
	brand := " Polymaker "
	material, err := service.Create(context.Background(), CreateMaterialInput{
		Name: " PLA Negro ", Type: " Filamento ", Brand: &brand, CostPerKG: 420.50, StockKG: 2.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if material.Name != "PLA Negro" || material.Type != "Filamento" || material.Brand == nil || *material.Brand != "Polymaker" || !material.Active {
		t.Fatalf("unexpected material: %+v", material)
	}

	stock := 3.25
	active := false
	updated, err := service.Update(context.Background(), material.ID, UpdateMaterialInput{
		StockKG: Field[float64]{Set: true, Value: &stock}, Active: Field[bool]{Set: true, Value: &active},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.StockKG != 3.25 || updated.Active {
		t.Fatalf("unexpected update: %+v", updated)
	}

	filterType := " Filamento "
	if _, err := service.List(context.Background(), MaterialFilters{Type: &filterType, Active: &active}); err != nil {
		t.Fatal(err)
	}
	if repository.filters.Type == nil || *repository.filters.Type != "Filamento" || repository.filters.Active == nil || *repository.filters.Active {
		t.Fatalf("unexpected filters: %+v", repository.filters)
	}
}

func TestMaterialServiceRejectsInvalidValues(t *testing.T) {
	service := NewMaterialService(&fakeMaterialRepository{})
	for _, input := range []CreateMaterialInput{
		{Type: "Filamento", CostPerKG: 1},
		{Name: "PLA", CostPerKG: 1},
		{Name: "PLA", Type: "Filamento", CostPerKG: -1},
		{Name: "PLA", Type: "Filamento", CostPerKG: 1, StockKG: -1},
	} {
		if _, err := service.Create(context.Background(), input); !isValidationError(err) {
			t.Fatalf("expected validation error for %+v, got %v", input, err)
		}
	}
}

type fakeRateRepository struct {
	machineExists  bool
	locationExists bool
	machineRate    models.MachineRate
	energyRate     models.EnergyRate
	existsErr      error
	rateErr        error
}

func (f *fakeRateRepository) MachineExists(context.Context, int64) (bool, error) {
	return f.machineExists, f.existsErr
}

func (f *fakeRateRepository) LocationExists(context.Context, int64) (bool, error) {
	return f.locationExists, f.existsErr
}

func (f *fakeRateRepository) GetMachineRate(context.Context, int64) (models.MachineRate, error) {
	return f.machineRate, f.rateErr
}

func (f *fakeRateRepository) UpsertMachineRate(_ context.Context, rate models.MachineRate) (models.MachineRate, error) {
	f.machineRate = rate
	return rate, f.rateErr
}

func (f *fakeRateRepository) GetEnergyRate(context.Context, int64) (models.EnergyRate, error) {
	return f.energyRate, f.rateErr
}

func (f *fakeRateRepository) UpsertEnergyRate(_ context.Context, rate models.EnergyRate) (models.EnergyRate, error) {
	f.energyRate = rate
	return rate, f.rateErr
}

func TestRateServiceUpsertsIndependentRates(t *testing.T) {
	repository := &fakeRateRepository{machineExists: true, locationExists: true}
	service := NewRateService(repository)
	machineRate, err := service.UpsertMachineRate(context.Background(), 3, MachineRateInput{
		InternalCostHour: 25, SalePriceHour: 70, PreparationCost: 15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if machineRate.MachineID != 3 || machineRate.SalePriceHour != 70 {
		t.Fatalf("unexpected machine rate: %+v", machineRate)
	}
	energyRate, err := service.UpsertEnergyRate(context.Background(), 5, EnergyRateInput{CostPerKWh: 2.3456})
	if err != nil {
		t.Fatal(err)
	}
	if energyRate.LocationID != 5 || energyRate.CostPerKWh != 2.3456 {
		t.Fatalf("unexpected energy rate: %+v", energyRate)
	}
}

func TestRateServiceValidatesReferencesAndCosts(t *testing.T) {
	service := NewRateService(&fakeRateRepository{})
	if _, err := service.UpsertMachineRate(context.Background(), 99, MachineRateInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing machine, got %v", err)
	}
	if _, err := service.UpsertEnergyRate(context.Background(), 99, EnergyRateInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing location, got %v", err)
	}
	service = NewRateService(&fakeRateRepository{machineExists: true, locationExists: true})
	if _, err := service.UpsertMachineRate(context.Background(), 1, MachineRateInput{SalePriceHour: -1}); !isValidationError(err) {
		t.Fatalf("expected rate validation error, got %v", err)
	}
	if _, err := service.UpsertEnergyRate(context.Background(), 1, EnergyRateInput{CostPerKWh: -1}); !isValidationError(err) {
		t.Fatalf("expected energy validation error, got %v", err)
	}
}

func TestRateServiceReturnsNilForUnconfiguredRates(t *testing.T) {
	repository := &fakeRateRepository{machineExists: true, locationExists: true, rateErr: repository.ErrNotFound}
	service := NewRateService(repository)
	machineRate, err := service.GetMachineRate(context.Background(), 1)
	if err != nil || machineRate != nil {
		t.Fatalf("expected unconfigured machine rate, got rate=%+v err=%v", machineRate, err)
	}
	energyRate, err := service.GetEnergyRate(context.Background(), 1)
	if err != nil || energyRate != nil {
		t.Fatalf("expected unconfigured energy rate, got rate=%+v err=%v", energyRate, err)
	}
}
