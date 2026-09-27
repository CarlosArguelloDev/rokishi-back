package service

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var orderStatuses = map[string]bool{
	"PENDIENTE": true, "EN_PRODUCCION": true, "COMPLETADO": true, "CANCELADO": true,
}

type OrderFilters struct {
	CustomerID *int64
	Status     *string
}

type FinishWorkInput struct {
	Result           string
	ConsumedMaterial string
	Waste            string
	Notes            *string
}

type productionRepository interface {
	CreateOrder(context.Context, int64) (models.Order, error)
	ListOrders(context.Context, repository.OrderFilters) ([]models.Order, error)
	GetOrder(context.Context, int64) (models.Order, error)
	GetWork(context.Context, int64) (models.Work, error)
	GetMachineAssignmentData(context.Context, int64) (repository.MachineAssignmentData, error)
	AssignMachine(context.Context, int64, int64) (models.Work, error)
	StartWork(context.Context, int64, time.Time) error
	FinishWork(context.Context, int64, repository.FinishWorkData) error
}

type ProductionService struct {
	repository productionRepository
	now        func() time.Time
}

func NewProductionService(repository productionRepository) *ProductionService {
	return &ProductionService{repository: repository, now: time.Now}
}

func (s *ProductionService) CreateOrder(ctx context.Context, quoteID int64) (models.Order, error) {
	if quoteID < 1 {
		return models.Order{}, &ValidationError{Message: "El identificador de cotizacion debe ser un entero positivo"}
	}
	order, err := s.repository.CreateOrder(ctx, quoteID)
	switch {
	case errors.Is(err, repository.ErrInvalidOperation):
		return models.Order{}, &ValidationError{Message: "Solo una cotizacion aceptada puede convertirse en pedido"}
	case errors.Is(err, repository.ErrConflict):
		return models.Order{}, ErrConflict
	case errors.Is(err, repository.ErrReferenceMissing):
		return models.Order{}, &ValidationError{Message: "La cotizacion no contiene conceptos validos"}
	default:
		return order, mapRepositoryError(err)
	}
}

func (s *ProductionService) ListOrders(ctx context.Context, filters OrderFilters) ([]models.Order, error) {
	if filters.CustomerID != nil && *filters.CustomerID < 1 {
		return nil, &ValidationError{Message: "El filtro cliente_id debe ser un entero positivo"}
	}
	if filters.Status != nil {
		status := strings.ToUpper(strings.TrimSpace(*filters.Status))
		if !orderStatuses[status] {
			return nil, &ValidationError{Message: "El filtro estado no es valido"}
		}
		filters.Status = &status
	}
	return s.repository.ListOrders(ctx, repository.OrderFilters{CustomerID: filters.CustomerID, Status: filters.Status})
}

func (s *ProductionService) GetOrder(ctx context.Context, id int64) (models.Order, error) {
	if id < 1 {
		return models.Order{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	order, err := s.repository.GetOrder(ctx, id)
	return order, mapRepositoryError(err)
}

func (s *ProductionService) AssignMachine(ctx context.Context, workID, machineID int64) (models.Work, error) {
	if workID < 1 || machineID < 1 {
		return models.Work{}, &ValidationError{Message: "Los identificadores de trabajo y maquina deben ser enteros positivos"}
	}
	work, err := s.repository.GetWork(ctx, workID)
	if err != nil {
		return models.Work{}, mapRepositoryError(err)
	}
	if work.Status != "PENDIENTE" {
		return models.Work{}, &ValidationError{Message: "Solo se puede reasignar un trabajo pendiente"}
	}
	machine, err := s.repository.GetMachineAssignmentData(ctx, machineID)
	if err != nil {
		return models.Work{}, mapRepositoryError(err)
	}
	if !machine.Active {
		return models.Work{}, &ValidationError{Message: "La maquina seleccionada esta inactiva"}
	}
	if machine.MachineTypeID != work.RequiredMachineTypeID {
		return models.Work{}, &ValidationError{Message: "La maquina seleccionada no es compatible con el trabajo"}
	}
	assigned, err := s.repository.AssignMachine(ctx, workID, machineID)
	if errors.Is(err, repository.ErrInvalidOperation) {
		return models.Work{}, &ValidationError{Message: "El trabajo ya no esta pendiente"}
	}
	return assigned, mapRepositoryError(err)
}

func (s *ProductionService) StartWork(ctx context.Context, workID int64) (models.Work, error) {
	if workID < 1 {
		return models.Work{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	work, err := s.repository.GetWork(ctx, workID)
	if err != nil {
		return models.Work{}, mapRepositoryError(err)
	}
	if work.Status != "PENDIENTE" || work.MachineID == nil {
		return models.Work{}, &ValidationError{Message: "El trabajo debe estar pendiente y tener una maquina asignada"}
	}
	if err := s.repository.StartWork(ctx, workID, s.now()); err != nil {
		if errors.Is(err, repository.ErrMachineUnavailable) || errors.Is(err, repository.ErrConflict) {
			return models.Work{}, &ValidationError{Message: "La maquina no esta disponible para iniciar este trabajo"}
		}
		if errors.Is(err, repository.ErrInvalidOperation) {
			return models.Work{}, &ValidationError{Message: "El trabajo ya no se puede iniciar"}
		}
		return models.Work{}, mapRepositoryError(err)
	}
	return s.repository.GetWork(ctx, workID)
}

func (s *ProductionService) FinishWork(ctx context.Context, workID int64, input FinishWorkInput) (models.Work, error) {
	if workID < 1 {
		return models.Work{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	result := strings.ToUpper(strings.TrimSpace(input.Result))
	if result != "EXITOSO" && result != "FALLIDO" {
		return models.Work{}, &ValidationError{Message: "El resultado debe ser EXITOSO o FALLIDO"}
	}
	consumed, err := parseProductionDecimal(input.ConsumedMaterial, "material_consumido_gramos")
	if err != nil {
		return models.Work{}, err
	}
	waste, err := parseProductionDecimal(input.Waste, "desperdicio_gramos")
	if err != nil {
		return models.Work{}, err
	}
	notes := cleanNullable(input.Notes)
	if notes != nil && utf8.RuneCountInString(*notes) > 2000 {
		return models.Work{}, maxLength("notas", 2000)
	}
	work, err := s.repository.GetWork(ctx, workID)
	if err != nil {
		return models.Work{}, mapRepositoryError(err)
	}
	if work.Status != "EN_PROCESO" {
		return models.Work{}, &ValidationError{Message: "El trabajo no tiene un intento en proceso"}
	}
	err = s.repository.FinishWork(ctx, workID, repository.FinishWorkData{
		Result: result, ConsumedMaterial: consumed, Waste: waste, Notes: notes, FinishedAt: s.now(),
	})
	if errors.Is(err, repository.ErrInvalidOperation) {
		return models.Work{}, &ValidationError{Message: "El intento o el estado operativo de la maquina cambio; vuelve a consultar el pedido"}
	}
	if err != nil {
		return models.Work{}, mapRepositoryError(err)
	}
	return s.repository.GetWork(ctx, workID)
}

func parseProductionDecimal(value, field string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if !decimalPattern.MatchString(cleaned) {
		return "", &ValidationError{Message: "El campo " + field + " debe ser un numero con hasta 3 decimales"}
	}
	parsed, ok := new(big.Rat).SetString(cleaned)
	if !ok || parsed.Sign() < 0 {
		return "", &ValidationError{Message: "El campo " + field + " debe ser mayor o igual a cero"}
	}
	if parsed.Cmp(big.NewRat(999999999999, 1000)) > 0 {
		return "", &ValidationError{Message: "El campo " + field + " excede el limite permitido"}
	}
	return cleaned, nil
}
