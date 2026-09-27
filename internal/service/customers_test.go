package service

import (
	"context"
	"testing"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeCustomerRepository struct {
	customer models.Customer
	filters  repository.CustomerFilters
	err      error
}

func (f *fakeCustomerRepository) Create(_ context.Context, customer models.Customer) (models.Customer, error) {
	customer.ID = 1
	f.customer = customer
	return customer, f.err
}

func (f *fakeCustomerRepository) List(_ context.Context, filters repository.CustomerFilters) ([]models.Customer, error) {
	f.filters = filters
	return []models.Customer{f.customer}, f.err
}

func (f *fakeCustomerRepository) Get(context.Context, int64) (models.Customer, error) {
	return f.customer, f.err
}

func (f *fakeCustomerRepository) Update(_ context.Context, customer models.Customer) (models.Customer, error) {
	f.customer = customer
	return customer, f.err
}

func TestCustomerServiceCreatesUpdatesAndFilters(t *testing.T) {
	repository := &fakeCustomerRepository{}
	customerService := NewCustomerService(repository)
	email := " ventas@example.com "
	phone := " 555-0100 "
	customer, err := customerService.Create(context.Background(), CreateCustomerInput{
		Name: " Taller Norte ", Email: &email, Phone: &phone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if customer.Type != "PERSONA" || customer.Name != "Taller Norte" || customer.Email == nil || *customer.Email != "ventas@example.com" || !customer.Active {
		t.Fatalf("unexpected customer: %+v", customer)
	}

	active := false
	updated, err := customerService.Update(context.Background(), customer.ID, UpdateCustomerInput{
		Active: Field[bool]{Set: true, Value: &active},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active {
		t.Fatalf("expected inactive customer: %+v", updated)
	}

	query := " Norte "
	if _, err := customerService.List(context.Background(), CustomerFilters{Query: &query}); err != nil {
		t.Fatal(err)
	}
	if repository.filters.Query == nil || *repository.filters.Query != "Norte" {
		t.Fatalf("unexpected filters: %+v", repository.filters)
	}
}

func TestCustomerServiceRejectsInvalidData(t *testing.T) {
	customerService := NewCustomerService(&fakeCustomerRepository{})
	invalidEmail := "not-an-email"
	for _, input := range []CreateCustomerInput{
		{},
		{Name: "Cliente", Email: &invalidEmail},
		{Name: "Cliente", Type: "MAYORISTA"},
	} {
		if _, err := customerService.Create(context.Background(), input); !isValidationError(err) {
			t.Fatalf("expected validation error for %+v, got %v", input, err)
		}
	}
}
