package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var (
	ErrNotFound = errors.New("registro no encontrado")
	ErrConflict = errors.New("registro duplicado")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type Field[T any] struct {
	Set   bool
	Value *T
}

type CreateLocationInput struct {
	Code     string
	Name     string
	Address  *string
	City     *string
	State    *string
	Timezone *string
}

type UpdateLocationInput struct {
	Code     Field[string]
	Name     Field[string]
	Address  Field[string]
	City     Field[string]
	State    Field[string]
	Timezone Field[string]
	Active   Field[bool]
}

type locationRepository interface {
	Create(context.Context, models.Location) (models.Location, error)
	List(context.Context) ([]models.Location, error)
	Get(context.Context, int64) (models.Location, error)
	Update(context.Context, models.Location) (models.Location, error)
}

type LocationService struct {
	repository locationRepository
}

func NewLocationService(repository locationRepository) *LocationService {
	return &LocationService{repository: repository}
}

func (s *LocationService) Create(ctx context.Context, input CreateLocationInput) (models.Location, error) {
	location := models.Location{
		Code:     strings.TrimSpace(input.Code),
		Name:     strings.TrimSpace(input.Name),
		Address:  cleanNullable(input.Address),
		City:     cleanNullable(input.City),
		State:    cleanNullable(input.State),
		Timezone: "America/Mexico_City",
		Active:   true,
	}
	if input.Timezone != nil {
		location.Timezone = strings.TrimSpace(*input.Timezone)
	}
	if err := validateLocation(location); err != nil {
		return models.Location{}, err
	}
	created, err := s.repository.Create(ctx, location)
	return created, mapRepositoryError(err)
}

func (s *LocationService) List(ctx context.Context) ([]models.Location, error) {
	return s.repository.List(ctx)
}

func (s *LocationService) Get(ctx context.Context, id int64) (models.Location, error) {
	location, err := s.repository.Get(ctx, id)
	return location, mapRepositoryError(err)
}

func (s *LocationService) Update(ctx context.Context, id int64, input UpdateLocationInput) (models.Location, error) {
	if !input.Code.Set && !input.Name.Set && !input.Address.Set && !input.City.Set && !input.State.Set &&
		!input.Timezone.Set && !input.Active.Set {
		return models.Location{}, &ValidationError{Message: "Debes proporcionar al menos un campo"}
	}
	location, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.Location{}, mapRepositoryError(err)
	}
	if input.Code.Set {
		if input.Code.Value == nil {
			return models.Location{}, requiredField("codigo")
		}
		location.Code = strings.TrimSpace(*input.Code.Value)
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.Location{}, requiredField("nombre")
		}
		location.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Address.Set {
		location.Address = cleanNullable(input.Address.Value)
	}
	if input.City.Set {
		location.City = cleanNullable(input.City.Value)
	}
	if input.State.Set {
		location.State = cleanNullable(input.State.Value)
	}
	if input.Timezone.Set {
		if input.Timezone.Value == nil {
			return models.Location{}, requiredField("zona_horaria")
		}
		location.Timezone = strings.TrimSpace(*input.Timezone.Value)
	}
	if input.Active.Set {
		if input.Active.Value == nil {
			return models.Location{}, requiredField("activa")
		}
		location.Active = *input.Active.Value
	}
	if err := validateLocation(location); err != nil {
		return models.Location{}, err
	}
	updated, err := s.repository.Update(ctx, location)
	return updated, mapRepositoryError(err)
}

type CreateMachineTypeInput struct {
	Name        string
	Description *string
}

type UpdateMachineTypeInput struct {
	Name        Field[string]
	Description Field[string]
}

type machineTypeRepository interface {
	Create(context.Context, models.MachineType) (models.MachineType, error)
	List(context.Context) ([]models.MachineType, error)
	Get(context.Context, int64) (models.MachineType, error)
	Update(context.Context, models.MachineType) (models.MachineType, error)
}

type MachineTypeService struct {
	repository machineTypeRepository
}

func NewMachineTypeService(repository machineTypeRepository) *MachineTypeService {
	return &MachineTypeService{repository: repository}
}

func (s *MachineTypeService) Create(ctx context.Context, input CreateMachineTypeInput) (models.MachineType, error) {
	machineType := models.MachineType{
		Name:        strings.TrimSpace(input.Name),
		Description: cleanNullable(input.Description),
	}
	if err := validateMachineType(machineType); err != nil {
		return models.MachineType{}, err
	}
	created, err := s.repository.Create(ctx, machineType)
	return created, mapRepositoryError(err)
}

func (s *MachineTypeService) List(ctx context.Context) ([]models.MachineType, error) {
	return s.repository.List(ctx)
}

func (s *MachineTypeService) Get(ctx context.Context, id int64) (models.MachineType, error) {
	machineType, err := s.repository.Get(ctx, id)
	return machineType, mapRepositoryError(err)
}

func (s *MachineTypeService) Update(ctx context.Context, id int64, input UpdateMachineTypeInput) (models.MachineType, error) {
	if !input.Name.Set && !input.Description.Set {
		return models.MachineType{}, &ValidationError{Message: "Debes proporcionar al menos un campo"}
	}
	machineType, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.MachineType{}, mapRepositoryError(err)
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.MachineType{}, requiredField("nombre")
		}
		machineType.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Description.Set {
		machineType.Description = cleanNullable(input.Description.Value)
	}
	if err := validateMachineType(machineType); err != nil {
		return models.MachineType{}, err
	}
	updated, err := s.repository.Update(ctx, machineType)
	return updated, mapRepositoryError(err)
}

func validateLocation(location models.Location) error {
	if location.Code == "" {
		return requiredField("codigo")
	}
	if utf8.RuneCountInString(location.Code) > 20 {
		return maxLength("codigo", 20)
	}
	if location.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(location.Name) > 100 {
		return maxLength("nombre", 100)
	}
	if location.City != nil && utf8.RuneCountInString(*location.City) > 100 {
		return maxLength("ciudad", 100)
	}
	if location.State != nil && utf8.RuneCountInString(*location.State) > 100 {
		return maxLength("estado", 100)
	}
	if location.Timezone == "" {
		return requiredField("zona_horaria")
	}
	if utf8.RuneCountInString(location.Timezone) > 50 {
		return maxLength("zona_horaria", 50)
	}
	return nil
}

func validateMachineType(machineType models.MachineType) error {
	if machineType.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(machineType.Name) > 100 {
		return maxLength("nombre", 100)
	}
	return nil
}

func cleanNullable(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}

func requiredField(field string) error {
	return &ValidationError{Message: fmt.Sprintf("El campo %s es obligatorio", field)}
}

func maxLength(field string, maximum int) error {
	return &ValidationError{Message: fmt.Sprintf("El campo %s no puede exceder %d caracteres", field, maximum)}
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, repository.ErrConflict):
		return ErrConflict
	default:
		return err
	}
}
