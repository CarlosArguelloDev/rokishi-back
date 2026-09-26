package service

import (
	"context"
	"errors"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeLocationRepository struct {
	location models.Location
	err      error
}

func (f *fakeLocationRepository) Create(_ context.Context, location models.Location) (models.Location, error) {
	location.ID = 1
	f.location = location
	return location, f.err
}

func (f *fakeLocationRepository) List(context.Context) ([]models.Location, error) {
	return []models.Location{f.location}, f.err
}

func (f *fakeLocationRepository) Get(context.Context, int64) (models.Location, error) {
	return f.location, f.err
}

func (f *fakeLocationRepository) Update(_ context.Context, location models.Location) (models.Location, error) {
	f.location = location
	return location, f.err
}

func TestLocationServiceCreateAndUpdate(t *testing.T) {
	repository := &fakeLocationRepository{}
	service := NewLocationService(repository)

	created, err := service.Create(context.Background(), CreateLocationInput{Code: " MX-01 ", Name: " Taller Centro "})
	if err != nil {
		t.Fatal(err)
	}
	if created.Code != "MX-01" || created.Name != "Taller Centro" || !created.Active || created.Timezone != "America/Mexico_City" {
		t.Fatalf("unexpected location: %+v", created)
	}

	active := false
	city := " Guadalajara "
	updated, err := service.Update(context.Background(), created.ID, UpdateLocationInput{
		Active: Field[bool]{Set: true, Value: &active},
		City:   Field[string]{Set: true, Value: &city},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active || updated.City == nil || *updated.City != "Guadalajara" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	updated, err = service.Update(context.Background(), created.ID, UpdateLocationInput{
		City: Field[string]{Set: true, Value: nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.City != nil {
		t.Fatalf("city was not cleared: %+v", updated)
	}
}

func TestLocationServiceErrors(t *testing.T) {
	service := NewLocationService(&fakeLocationRepository{})
	if _, err := service.Create(context.Background(), CreateLocationInput{}); !isValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if _, err := service.Update(context.Background(), 1, UpdateLocationInput{}); !isValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	conflictService := NewLocationService(&fakeLocationRepository{err: repository.ErrConflict})
	if _, err := conflictService.Create(context.Background(), CreateLocationInput{Code: "MX", Name: "Centro"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	notFoundService := NewLocationService(&fakeLocationRepository{err: repository.ErrNotFound})
	if _, err := notFoundService.Get(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

type fakeMachineTypeRepository struct {
	machineType models.MachineType
	err         error
}

func (f *fakeMachineTypeRepository) Create(_ context.Context, machineType models.MachineType) (models.MachineType, error) {
	machineType.ID = 1
	f.machineType = machineType
	return machineType, f.err
}

func (f *fakeMachineTypeRepository) List(context.Context) ([]models.MachineType, error) {
	return []models.MachineType{f.machineType}, f.err
}

func (f *fakeMachineTypeRepository) Get(context.Context, int64) (models.MachineType, error) {
	return f.machineType, f.err
}

func (f *fakeMachineTypeRepository) Update(_ context.Context, machineType models.MachineType) (models.MachineType, error) {
	f.machineType = machineType
	return machineType, f.err
}

func TestMachineTypeServiceCreateAndUpdate(t *testing.T) {
	repository := &fakeMachineTypeRepository{}
	service := NewMachineTypeService(repository)
	description := " Deposicion de filamento "
	created, err := service.Create(context.Background(), CreateMachineTypeInput{Name: " FDM ", Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "FDM" || created.Description == nil || *created.Description != "Deposicion de filamento" {
		t.Fatalf("unexpected machine type: %+v", created)
	}

	updated, err := service.Update(context.Background(), created.ID, UpdateMachineTypeInput{
		Description: Field[string]{Set: true, Value: nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != nil {
		t.Fatalf("description was not cleared: %+v", updated)
	}
}

func isValidationError(err error) bool {
	var validationError *ValidationError
	return errors.As(err, &validationError)
}
