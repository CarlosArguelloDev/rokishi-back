package service

import (
	"context"
	"errors"
	"math/big"
	"strconv"
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

type CreateDirectWorkInput struct {
	Description           *string
	RequiredMachineTypeID int64
	MachineID             *int64
	MaterialID            int64
	PieceCount            int64
	EstimatedMinutes      int64
	EstimatedMaterial     string
}

type CreateDirectOrderInput struct {
	CustomerID    int64
	SalesPlatform *string
	Notes         *string
	Works         []CreateDirectWorkInput
}

type productionRepository interface {
	CreateOrder(context.Context, int64) (models.Order, error)
	GetDirectOrderCustomerData(context.Context, int64) (repository.DirectOrderCustomerData, error)
	GetDirectWorkReferenceData(context.Context, int64, int64, *int64) (repository.DirectWorkReferenceData, error)
	CreateDirectOrder(context.Context, repository.CreateDirectOrderData) (models.Order, error)
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

func (s *ProductionService) CreateDirectOrder(ctx context.Context, input CreateDirectOrderInput) (models.Order, error) {
	if input.CustomerID < 1 {
		return models.Order{}, &ValidationError{Message: "El identificador de cliente debe ser un entero positivo"}
	}
	customer, err := s.repository.GetDirectOrderCustomerData(ctx, input.CustomerID)
	if err != nil {
		return models.Order{}, mapRepositoryError(err)
	}
	if !customer.Active {
		return models.Order{}, &ValidationError{Message: "El cliente seleccionado esta inactivo"}
	}
	if len(input.Works) == 0 || len(input.Works) > 50 {
		return models.Order{}, &ValidationError{Message: "El pedido debe incluir entre 1 y 50 trabajos"}
	}

	platform := cleanNullable(input.SalesPlatform)
	if platform != nil && utf8.RuneCountInString(*platform) > 100 {
		return models.Order{}, maxLength("plataforma_venta", 100)
	}
	origin := "CLIENTE"
	if customer.Type == "EMPRESA" {
		origin = "EMPRESA"
	}
	if platform != nil {
		origin = "PLATAFORMA"
	}
	notes := cleanNullable(input.Notes)
	if notes != nil && utf8.RuneCountInString(*notes) > 2000 {
		return models.Order{}, maxLength("notas", 2000)
	}

	works := make([]repository.DirectWorkData, 0, len(input.Works))
	for index, inputWork := range input.Works {
		fieldPrefix := "trabajos[" + strconv.Itoa(index) + "]"
		if inputWork.RequiredMachineTypeID < 1 || inputWork.MaterialID < 1 {
			return models.Order{}, &ValidationError{Message: fieldPrefix + " requiere tipo de maquina y material validos"}
		}
		if inputWork.MachineID != nil && *inputWork.MachineID < 1 {
			return models.Order{}, &ValidationError{Message: fieldPrefix + ".maquina_id debe ser un entero positivo"}
		}
		if inputWork.PieceCount < 1 || inputWork.PieceCount > 1000000 {
			return models.Order{}, &ValidationError{Message: fieldPrefix + ".cantidad_piezas debe estar entre 1 y 1000000"}
		}
		if inputWork.EstimatedMinutes < 1 || inputWork.EstimatedMinutes > 5256000 {
			return models.Order{}, &ValidationError{Message: fieldPrefix + ".duracion_estimada_minutos debe estar entre 1 y 5256000"}
		}
		estimatedMaterial, err := parseProductionDecimal(inputWork.EstimatedMaterial, fieldPrefix+".material_estimado_gramos")
		if err != nil {
			return models.Order{}, err
		}
		parsedMaterial, _ := new(big.Rat).SetString(estimatedMaterial)
		if parsedMaterial.Sign() <= 0 {
			return models.Order{}, &ValidationError{Message: "El campo " + fieldPrefix + ".material_estimado_gramos debe ser mayor a cero"}
		}
		description := cleanNullable(inputWork.Description)
		if description != nil && utf8.RuneCountInString(*description) > 200 {
			return models.Order{}, maxLength(fieldPrefix+".descripcion", 200)
		}

		references, err := s.repository.GetDirectWorkReferenceData(ctx, inputWork.RequiredMachineTypeID, inputWork.MaterialID, inputWork.MachineID)
		if err != nil {
			return models.Order{}, mapRepositoryError(err)
		}
		if !references.MachineTypeExists {
			return models.Order{}, &ValidationError{Message: fieldPrefix + " usa un tipo de maquina inexistente"}
		}
		if !references.MaterialActive {
			return models.Order{}, &ValidationError{Message: fieldPrefix + " usa un material inexistente o inactivo"}
		}
		if inputWork.MachineID != nil {
			if !references.MachineExists || !references.MachineActive {
				return models.Order{}, &ValidationError{Message: fieldPrefix + " usa una maquina inexistente o inactiva"}
			}
			if references.MachineTypeID == nil || *references.MachineTypeID != inputWork.RequiredMachineTypeID {
				return models.Order{}, &ValidationError{Message: fieldPrefix + " usa una maquina incompatible con el tipo requerido"}
			}
		}
		works = append(works, repository.DirectWorkData{
			Description: description, RequiredMachineTypeID: inputWork.RequiredMachineTypeID,
			MachineID: inputWork.MachineID, MaterialID: inputWork.MaterialID,
			PieceCount: inputWork.PieceCount, EstimatedMinutes: inputWork.EstimatedMinutes,
			EstimatedMaterial: estimatedMaterial,
		})
	}

	order, err := s.repository.CreateDirectOrder(ctx, repository.CreateDirectOrderData{
		CustomerID: input.CustomerID, Origin: origin, SalesPlatform: platform, Notes: notes, Works: works,
	})
	return order, mapRepositoryError(err)
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
