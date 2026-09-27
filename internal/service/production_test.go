package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeProductionRepository struct {
	order      models.Order
	work       models.Work
	machine    repository.MachineAssignmentData
	customer   repository.DirectOrderCustomerData
	references repository.DirectWorkReferenceData
	directData *repository.CreateDirectOrderData
	err        error
	started    bool
	finished   *repository.FinishWorkData
	assignedID int64
}

func (f *fakeProductionRepository) CreateOrder(context.Context, int64) (models.Order, error) {
	return f.order, f.err
}

func (f *fakeProductionRepository) GetDirectOrderCustomerData(context.Context, int64) (repository.DirectOrderCustomerData, error) {
	return f.customer, f.err
}

func (f *fakeProductionRepository) GetDirectWorkReferenceData(context.Context, int64, int64, *int64) (repository.DirectWorkReferenceData, error) {
	return f.references, f.err
}

func (f *fakeProductionRepository) CreateDirectOrder(_ context.Context, data repository.CreateDirectOrderData) (models.Order, error) {
	f.directData = &data
	return f.order, f.err
}

func (f *fakeProductionRepository) ListOrders(context.Context, repository.OrderFilters) ([]models.Order, error) {
	return []models.Order{f.order}, f.err
}

func (f *fakeProductionRepository) GetOrder(context.Context, int64) (models.Order, error) {
	return f.order, f.err
}

func (f *fakeProductionRepository) GetWork(context.Context, int64) (models.Work, error) {
	return f.work, f.err
}

func (f *fakeProductionRepository) GetMachineAssignmentData(context.Context, int64) (repository.MachineAssignmentData, error) {
	return f.machine, f.err
}

func (f *fakeProductionRepository) AssignMachine(_ context.Context, _ int64, machineID int64) (models.Work, error) {
	f.assignedID = machineID
	return f.work, f.err
}

func (f *fakeProductionRepository) StartWork(context.Context, int64, time.Time) error {
	f.started = true
	return f.err
}

func (f *fakeProductionRepository) FinishWork(_ context.Context, _ int64, data repository.FinishWorkData) error {
	f.finished = &data
	return f.err
}

func TestProductionServiceCreatesOrderFromAcceptedQuote(t *testing.T) {
	quoteID := int64(7)
	repo := &fakeProductionRepository{order: models.Order{ID: 4, QuoteID: &quoteID, Status: "PENDIENTE"}}
	production := NewProductionService(repo)
	order, err := production.CreateOrder(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != 4 || order.QuoteID == nil || *order.QuoteID != 7 {
		t.Fatalf("unexpected order: %+v", order)
	}

	repo.err = repository.ErrInvalidOperation
	if _, err := production.CreateOrder(context.Background(), 7); !isValidationError(err) {
		t.Fatalf("expected invalid quote state, got %v", err)
	}
	repo.err = repository.ErrConflict
	if _, err := production.CreateOrder(context.Background(), 7); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected duplicate order conflict, got %v", err)
	}
}

func TestProductionServiceCreatesDirectPlatformOrder(t *testing.T) {
	machineID := int64(6)
	platform := " Mercado Libre "
	repo := &fakeProductionRepository{
		order:      models.Order{ID: 12, Origin: "PLATAFORMA"},
		customer:   repository.DirectOrderCustomerData{Type: "PERSONA", Active: true},
		references: repository.DirectWorkReferenceData{MachineTypeExists: true, MaterialActive: true, MachineExists: true, MachineActive: true, MachineTypeID: int64Pointer(2)},
	}
	production := NewProductionService(repo)
	order, err := production.CreateDirectOrder(context.Background(), CreateDirectOrderInput{
		CustomerID: 3, SalesPlatform: &platform,
		Works: []CreateDirectWorkInput{{
			Description: intStringPointer(" Lote ecommerce "), RequiredMachineTypeID: 2,
			MachineID: &machineID, MaterialID: 4, PieceCount: 3,
			EstimatedMinutes: 90, EstimatedMaterial: "125.500",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != 12 || repo.directData == nil || repo.directData.Origin != "PLATAFORMA" {
		t.Fatalf("unexpected direct order data: %+v", repo.directData)
	}
	if repo.directData.SalesPlatform == nil || *repo.directData.SalesPlatform != "Mercado Libre" {
		t.Fatalf("unexpected platform: %+v", repo.directData.SalesPlatform)
	}
	if repo.directData.Works[0].Description == nil || *repo.directData.Works[0].Description != "Lote ecommerce" {
		t.Fatalf("unexpected work: %+v", repo.directData.Works[0])
	}
}

func TestProductionServiceRejectsInvalidDirectOrder(t *testing.T) {
	repo := &fakeProductionRepository{customer: repository.DirectOrderCustomerData{Type: "EMPRESA", Active: true}}
	production := NewProductionService(repo)
	if _, err := production.CreateDirectOrder(context.Background(), CreateDirectOrderInput{CustomerID: 2}); !isValidationError(err) {
		t.Fatalf("expected empty work validation, got %v", err)
	}
	repo.customer.Active = false
	if _, err := production.CreateDirectOrder(context.Background(), CreateDirectOrderInput{CustomerID: 2, Works: []CreateDirectWorkInput{{}}}); !isValidationError(err) {
		t.Fatalf("expected inactive customer validation, got %v", err)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func intStringPointer(value string) *string { return &value }

func TestProductionServiceRequiresCompatibleActiveMachine(t *testing.T) {
	repo := &fakeProductionRepository{
		work:    models.Work{ID: 9, Status: "PENDIENTE", RequiredMachineTypeID: 2},
		machine: repository.MachineAssignmentData{Active: true, MachineTypeID: 3},
	}
	production := NewProductionService(repo)
	if _, err := production.AssignMachine(context.Background(), 9, 6); !isValidationError(err) {
		t.Fatalf("expected incompatible machine validation, got %v", err)
	}
	repo.machine.MachineTypeID = 2
	if _, err := production.AssignMachine(context.Background(), 9, 6); err != nil {
		t.Fatal(err)
	}
	if repo.assignedID != 6 {
		t.Fatalf("assigned machine = %d, want 6", repo.assignedID)
	}
	repo.machine.Active = false
	if _, err := production.AssignMachine(context.Background(), 9, 6); !isValidationError(err) {
		t.Fatalf("expected inactive machine validation, got %v", err)
	}
}

func TestProductionServiceStartsAndFinishesWork(t *testing.T) {
	machineID := int64(6)
	repo := &fakeProductionRepository{work: models.Work{ID: 9, Status: "PENDIENTE", MachineID: &machineID}}
	production := NewProductionService(repo)
	production.now = func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) }
	if _, err := production.StartWork(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if !repo.started {
		t.Fatal("start was not persisted")
	}

	repo.work.Status = "EN_PROCESO"
	if _, err := production.FinishWork(context.Background(), 9, FinishWorkInput{
		Result: "fallido", ConsumedMaterial: "82.500", Waste: "12.250",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.finished == nil || repo.finished.Result != "FALLIDO" || repo.finished.Waste != "12.250" {
		t.Fatalf("unexpected finish data: %+v", repo.finished)
	}
}

func TestProductionServiceRejectsInvalidResultAndMeasurements(t *testing.T) {
	production := NewProductionService(&fakeProductionRepository{work: models.Work{ID: 9, Status: "EN_PROCESO"}})
	inputs := []FinishWorkInput{
		{Result: "CANCELADO", ConsumedMaterial: "10", Waste: "0"},
		{Result: "EXITOSO", ConsumedMaterial: "-1", Waste: "0"},
		{Result: "EXITOSO", ConsumedMaterial: "1.0001", Waste: "0"},
		{Result: "EXITOSO", ConsumedMaterial: "1", Waste: "-1"},
	}
	for _, input := range inputs {
		if _, err := production.FinishWork(context.Background(), 9, input); !isValidationError(err) {
			t.Fatalf("expected validation error for %+v, got %v", input, err)
		}
	}
}
