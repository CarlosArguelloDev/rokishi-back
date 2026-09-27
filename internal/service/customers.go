package service

import (
	"context"
	"net/mail"
	"strings"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type CreateCustomerInput struct {
	Name  string
	Email *string
	Phone *string
	Notes *string
}

type UpdateCustomerInput struct {
	Name   Field[string]
	Email  Field[string]
	Phone  Field[string]
	Notes  Field[string]
	Active Field[bool]
}

type CustomerFilters struct {
	Active *bool
	Query  *string
}

type customerRepository interface {
	Create(context.Context, models.Customer) (models.Customer, error)
	List(context.Context, repository.CustomerFilters) ([]models.Customer, error)
	Get(context.Context, int64) (models.Customer, error)
	Update(context.Context, models.Customer) (models.Customer, error)
}

type CustomerService struct {
	repository customerRepository
}

func NewCustomerService(repository customerRepository) *CustomerService {
	return &CustomerService{repository: repository}
}

func (s *CustomerService) Create(ctx context.Context, input CreateCustomerInput) (models.Customer, error) {
	customer := models.Customer{
		Name: strings.TrimSpace(input.Name), Email: cleanNullable(input.Email),
		Phone: cleanNullable(input.Phone), Notes: cleanNullable(input.Notes), Active: true,
	}
	if err := validateCustomer(customer); err != nil {
		return models.Customer{}, err
	}
	created, err := s.repository.Create(ctx, customer)
	return created, mapRepositoryError(err)
}

func (s *CustomerService) List(ctx context.Context, filters CustomerFilters) ([]models.Customer, error) {
	if filters.Query != nil {
		query := strings.TrimSpace(*filters.Query)
		if query == "" {
			filters.Query = nil
		} else {
			if utf8.RuneCountInString(query) > 100 {
				return nil, maxLength("q", 100)
			}
			filters.Query = &query
		}
	}
	return s.repository.List(ctx, repository.CustomerFilters{Active: filters.Active, Query: filters.Query})
}

func (s *CustomerService) Get(ctx context.Context, id int64) (models.Customer, error) {
	if id < 1 {
		return models.Customer{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	customer, err := s.repository.Get(ctx, id)
	return customer, mapRepositoryError(err)
}

func (s *CustomerService) Update(ctx context.Context, id int64, input UpdateCustomerInput) (models.Customer, error) {
	if !input.Name.Set && !input.Email.Set && !input.Phone.Set && !input.Notes.Set && !input.Active.Set {
		return models.Customer{}, &ValidationError{Message: "Debes proporcionar al menos un campo"}
	}
	customer, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.Customer{}, mapRepositoryError(err)
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.Customer{}, requiredField("nombre")
		}
		customer.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Email.Set {
		customer.Email = cleanNullable(input.Email.Value)
	}
	if input.Phone.Set {
		customer.Phone = cleanNullable(input.Phone.Value)
	}
	if input.Notes.Set {
		customer.Notes = cleanNullable(input.Notes.Value)
	}
	if input.Active.Set {
		if input.Active.Value == nil {
			return models.Customer{}, requiredField("activo")
		}
		customer.Active = *input.Active.Value
	}
	if err := validateCustomer(customer); err != nil {
		return models.Customer{}, err
	}
	updated, err := s.repository.Update(ctx, customer)
	return updated, mapRepositoryError(err)
}

func validateCustomer(customer models.Customer) error {
	if customer.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(customer.Name) > 100 {
		return maxLength("nombre", 100)
	}
	if customer.Email != nil {
		if utf8.RuneCountInString(*customer.Email) > 254 {
			return maxLength("correo", 254)
		}
		address, err := mail.ParseAddress(*customer.Email)
		if err != nil || address.Address != *customer.Email {
			return &ValidationError{Message: "El campo correo no tiene un formato valido"}
		}
	}
	if customer.Phone != nil && utf8.RuneCountInString(*customer.Phone) > 30 {
		return maxLength("telefono", 30)
	}
	if customer.Notes != nil && utf8.RuneCountInString(*customer.Notes) > 2000 {
		return maxLength("notas", 2000)
	}
	return nil
}
