package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type CreateMachineInput struct {
	LocationID    int64
	MachineTypeID int64
	Code          string
	Name          string
	Brand         *string
	Model         *string
	SerialNumber  *string
	PowerWatts    *float64
	PurchaseDate  *string
}

type UpdateMachineInput struct {
	LocationID    Field[int64]
	MachineTypeID Field[int64]
	Code          Field[string]
	Name          Field[string]
	Brand         Field[string]
	Model         Field[string]
	SerialNumber  Field[string]
	PowerWatts    Field[float64]
	PurchaseDate  Field[string]
	Active        Field[bool]
}

type MachineFilters struct {
	LocationID    *int64
	MachineTypeID *int64
	Active        *bool
	Query         *string
}

type machineRepository interface {
	Create(context.Context, models.Machine) (models.Machine, error)
	List(context.Context, repository.MachineFilters) ([]models.Machine, error)
	Get(context.Context, int64) (models.Machine, error)
	Update(context.Context, models.Machine) (models.Machine, error)
	LocationExists(context.Context, int64) (bool, error)
	MachineTypeExists(context.Context, int64) (bool, error)
}

type MachineService struct {
	repository machineRepository
}

func NewMachineService(repository machineRepository) *MachineService {
	return &MachineService{repository: repository}
}

func (s *MachineService) Create(ctx context.Context, input CreateMachineInput) (models.Machine, error) {
	machine := models.Machine{
		LocationID:    input.LocationID,
		MachineTypeID: input.MachineTypeID,
		Code:          strings.TrimSpace(input.Code),
		Name:          strings.TrimSpace(input.Name),
		Brand:         cleanNullable(input.Brand),
		Model:         cleanNullable(input.Model),
		SerialNumber:  cleanNullable(input.SerialNumber),
		PowerWatts:    input.PowerWatts,
		PurchaseDate:  cleanNullable(input.PurchaseDate),
		Active:        true,
	}
	if err := validateMachine(machine); err != nil {
		return models.Machine{}, err
	}
	if err := s.validateReferences(ctx, machine.LocationID, machine.MachineTypeID); err != nil {
		return models.Machine{}, err
	}
	created, err := s.repository.Create(ctx, machine)
	return created, mapMachineRepositoryError(err)
}

func (s *MachineService) List(ctx context.Context, filters MachineFilters) ([]models.Machine, error) {
	if filters.LocationID != nil && *filters.LocationID < 1 {
		return nil, &ValidationError{Message: "El filtro locacion_id debe ser un entero positivo"}
	}
	if filters.MachineTypeID != nil && *filters.MachineTypeID < 1 {
		return nil, &ValidationError{Message: "El filtro tipo_maquina_id debe ser un entero positivo"}
	}
	if filters.Query != nil {
		query := strings.TrimSpace(*filters.Query)
		if query == "" {
			filters.Query = nil
		} else {
			if utf8.RuneCountInString(query) > 100 {
				return nil, &ValidationError{Message: "El filtro q no puede exceder 100 caracteres"}
			}
			filters.Query = &query
		}
	}
	return s.repository.List(ctx, repository.MachineFilters{
		LocationID: filters.LocationID, MachineTypeID: filters.MachineTypeID,
		Active: filters.Active, Query: filters.Query,
	})
}

func (s *MachineService) Get(ctx context.Context, id int64) (models.Machine, error) {
	machine, err := s.repository.Get(ctx, id)
	return machine, mapRepositoryError(err)
}

func (s *MachineService) Update(ctx context.Context, id int64, input UpdateMachineInput) (models.Machine, error) {
	if !input.LocationID.Set && !input.MachineTypeID.Set && !input.Code.Set && !input.Name.Set &&
		!input.Brand.Set && !input.Model.Set && !input.SerialNumber.Set && !input.PowerWatts.Set &&
		!input.PurchaseDate.Set && !input.Active.Set {
		return models.Machine{}, &ValidationError{Message: "Debes proporcionar al menos un campo"}
	}
	machine, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.Machine{}, mapRepositoryError(err)
	}
	if input.LocationID.Set {
		if input.LocationID.Value == nil {
			return models.Machine{}, requiredField("locacion_id")
		}
		machine.LocationID = *input.LocationID.Value
	}
	if input.MachineTypeID.Set {
		if input.MachineTypeID.Value == nil {
			return models.Machine{}, requiredField("tipo_maquina_id")
		}
		machine.MachineTypeID = *input.MachineTypeID.Value
	}
	if input.Code.Set {
		if input.Code.Value == nil {
			return models.Machine{}, requiredField("codigo")
		}
		machine.Code = strings.TrimSpace(*input.Code.Value)
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.Machine{}, requiredField("nombre")
		}
		machine.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Brand.Set {
		machine.Brand = cleanNullable(input.Brand.Value)
	}
	if input.Model.Set {
		machine.Model = cleanNullable(input.Model.Value)
	}
	if input.SerialNumber.Set {
		machine.SerialNumber = cleanNullable(input.SerialNumber.Value)
	}
	if input.PowerWatts.Set {
		machine.PowerWatts = input.PowerWatts.Value
	}
	if input.PurchaseDate.Set {
		machine.PurchaseDate = cleanNullable(input.PurchaseDate.Value)
	}
	if input.Active.Set {
		if input.Active.Value == nil {
			return models.Machine{}, requiredField("activa")
		}
		machine.Active = *input.Active.Value
	}
	if err := validateMachine(machine); err != nil {
		return models.Machine{}, err
	}
	if err := s.validateReferences(ctx, machine.LocationID, machine.MachineTypeID); err != nil {
		return models.Machine{}, err
	}
	updated, err := s.repository.Update(ctx, machine)
	return updated, mapMachineRepositoryError(err)
}

func (s *MachineService) validateReferences(ctx context.Context, locationID, machineTypeID int64) error {
	locationExists, err := s.repository.LocationExists(ctx, locationID)
	if err != nil {
		return err
	}
	if !locationExists {
		return &ValidationError{Message: "La locacion indicada no existe"}
	}
	typeExists, err := s.repository.MachineTypeExists(ctx, machineTypeID)
	if err != nil {
		return err
	}
	if !typeExists {
		return &ValidationError{Message: "El tipo de maquina indicado no existe"}
	}
	return nil
}

func validateMachine(machine models.Machine) error {
	if machine.LocationID < 1 {
		return &ValidationError{Message: "El campo locacion_id debe ser un entero positivo"}
	}
	if machine.MachineTypeID < 1 {
		return &ValidationError{Message: "El campo tipo_maquina_id debe ser un entero positivo"}
	}
	if machine.Code == "" {
		return requiredField("codigo")
	}
	if utf8.RuneCountInString(machine.Code) > 30 {
		return maxLength("codigo", 30)
	}
	if machine.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(machine.Name) > 100 {
		return maxLength("nombre", 100)
	}
	for field, value := range map[string]*string{
		"marca": machine.Brand, "modelo": machine.Model, "numero_serie": machine.SerialNumber,
	} {
		if value != nil && utf8.RuneCountInString(*value) > 100 {
			return maxLength(field, 100)
		}
	}
	if machine.PowerWatts != nil && (*machine.PowerWatts < 0 || *machine.PowerWatts > 99999999.99 || math.IsNaN(*machine.PowerWatts) || math.IsInf(*machine.PowerWatts, 0)) {
		return &ValidationError{Message: "El campo potencia_watts debe estar entre 0 y 99999999.99"}
	}
	if machine.PurchaseDate != nil {
		if _, err := time.Parse("2006-01-02", *machine.PurchaseDate); err != nil {
			return &ValidationError{Message: "El campo fecha_compra debe tener formato YYYY-MM-DD"}
		}
	}
	return nil
}

func mapMachineRepositoryError(err error) error {
	if errors.Is(err, repository.ErrReferenceMissing) {
		return &ValidationError{Message: "La locacion o el tipo de maquina indicado ya no existe"}
	}
	return mapRepositoryError(err)
}
