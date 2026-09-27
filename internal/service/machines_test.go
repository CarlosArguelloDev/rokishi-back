package service

import (
	"context"
	"errors"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeMachineRepository struct {
	machine           models.Machine
	filters           repository.MachineFilters
	locationExists    bool
	machineTypeExists bool
	err               error
	referenceErr      error
}

func (f *fakeMachineRepository) Create(_ context.Context, machine models.Machine) (models.Machine, error) {
	machine.ID = 1
	f.machine = machine
	return machine, f.err
}

func (f *fakeMachineRepository) List(_ context.Context, filters repository.MachineFilters) ([]models.Machine, error) {
	f.filters = filters
	return []models.Machine{f.machine}, f.err
}

func (f *fakeMachineRepository) Get(context.Context, int64) (models.Machine, error) {
	return f.machine, f.err
}

func (f *fakeMachineRepository) Update(_ context.Context, machine models.Machine) (models.Machine, error) {
	f.machine = machine
	return machine, f.err
}

func (f *fakeMachineRepository) LocationExists(context.Context, int64) (bool, error) {
	return f.locationExists, f.referenceErr
}

func (f *fakeMachineRepository) MachineTypeExists(context.Context, int64) (bool, error) {
	return f.machineTypeExists, f.referenceErr
}

func TestMachineServiceCreateAndUpdate(t *testing.T) {
	repository := &fakeMachineRepository{locationExists: true, machineTypeExists: true}
	service := NewMachineService(repository)
	brand := " Prusa "
	power := 120.5
	machine, err := service.Create(context.Background(), CreateMachineInput{
		LocationID: 1, MachineTypeID: 2, Code: " PR-01 ", Name: " MK4 ", Brand: &brand, PowerWatts: &power,
	})
	if err != nil {
		t.Fatal(err)
	}
	if machine.Code != "PR-01" || machine.Name != "MK4" || machine.Brand == nil || *machine.Brand != "Prusa" || !machine.Active {
		t.Fatalf("unexpected machine: %+v", machine)
	}

	newLocationID := int64(3)
	active := false
	updated, err := service.Update(context.Background(), machine.ID, UpdateMachineInput{
		LocationID: Field[int64]{Set: true, Value: &newLocationID},
		Active:     Field[bool]{Set: true, Value: &active},
		Brand:      Field[string]{Set: true, Value: nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LocationID != 3 || updated.Active || updated.Brand != nil {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestMachineServiceValidatesReferencesAndFields(t *testing.T) {
	service := NewMachineService(&fakeMachineRepository{locationExists: false, machineTypeExists: true})
	_, err := service.Create(context.Background(), CreateMachineInput{
		LocationID: 99, MachineTypeID: 1, Code: "M-1", Name: "Maquina",
	})
	if !isValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	service = NewMachineService(&fakeMachineRepository{locationExists: true, machineTypeExists: true})
	negativePower := -1.0
	_, err = service.Create(context.Background(), CreateMachineInput{
		LocationID: 1, MachineTypeID: 1, Code: "M-1", Name: "Maquina", PowerWatts: &negativePower,
	})
	if !isValidationError(err) {
		t.Fatalf("expected power validation error, got %v", err)
	}

	invalidDate := "2026/01/01"
	_, err = service.Create(context.Background(), CreateMachineInput{
		LocationID: 1, MachineTypeID: 1, Code: "M-1", Name: "Maquina", PurchaseDate: &invalidDate,
	})
	if !isValidationError(err) {
		t.Fatalf("expected date validation error, got %v", err)
	}
}

func TestMachineServiceMapsErrorsAndFilters(t *testing.T) {
	conflictRepository := &fakeMachineRepository{
		locationExists: true, machineTypeExists: true, err: repository.ErrConflict,
	}
	service := NewMachineService(conflictRepository)
	_, err := service.Create(context.Background(), CreateMachineInput{
		LocationID: 1, MachineTypeID: 1, Code: "M-1", Name: "Maquina",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	repository := &fakeMachineRepository{locationExists: true, machineTypeExists: true}
	service = NewMachineService(repository)
	locationID := int64(4)
	active := true
	query := " prusa "
	if _, err := service.List(context.Background(), MachineFilters{
		LocationID: &locationID, Active: &active, Query: &query,
	}); err != nil {
		t.Fatal(err)
	}
	if repository.filters.LocationID == nil || *repository.filters.LocationID != 4 ||
		repository.filters.Active == nil || !*repository.filters.Active ||
		repository.filters.Query == nil || *repository.filters.Query != "prusa" {
		t.Fatalf("unexpected filters: %+v", repository.filters)
	}
}
