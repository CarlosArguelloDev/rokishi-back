package service

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type CreateMaterialInput struct {
	Name      string
	Type      string
	Brand     *string
	Color     *string
	CostPerKG float64
	StockKG   float64
}

type UpdateMaterialInput struct {
	Name      Field[string]
	Type      Field[string]
	Brand     Field[string]
	Color     Field[string]
	CostPerKG Field[float64]
	StockKG   Field[float64]
	Active    Field[bool]
}

type MaterialFilters struct {
	Type   *string
	Brand  *string
	Color  *string
	Active *bool
}

type MachineRateInput struct {
	InternalCostHour float64
	SalePriceHour    float64
	PreparationCost  float64
}

type EnergyRateInput struct {
	CostPerKWh float64
}

type materialRepository interface {
	Create(context.Context, models.Material) (models.Material, error)
	List(context.Context, repository.MaterialFilters) ([]models.Material, error)
	Get(context.Context, int64) (models.Material, error)
	Update(context.Context, models.Material) (models.Material, error)
}

type MaterialService struct {
	repository materialRepository
}

func NewMaterialService(repository materialRepository) *MaterialService {
	return &MaterialService{repository: repository}
}

func (s *MaterialService) Create(ctx context.Context, input CreateMaterialInput) (models.Material, error) {
	material := models.Material{
		Name: input.Name, Type: input.Type, Brand: input.Brand, Color: input.Color,
		CostPerKG: input.CostPerKG, StockKG: input.StockKG, Active: true,
	}
	cleanMaterial(&material)
	if err := validateMaterial(material); err != nil {
		return models.Material{}, err
	}
	created, err := s.repository.Create(ctx, material)
	return created, mapRepositoryError(err)
}

func (s *MaterialService) List(ctx context.Context, filters MaterialFilters) ([]models.Material, error) {
	cleaned := repository.MaterialFilters{Active: filters.Active}
	var err error
	if cleaned.Type, err = cleanFilter(filters.Type, "tipo", 50); err != nil {
		return nil, err
	}
	if cleaned.Brand, err = cleanFilter(filters.Brand, "marca", 100); err != nil {
		return nil, err
	}
	if cleaned.Color, err = cleanFilter(filters.Color, "color", 50); err != nil {
		return nil, err
	}
	return s.repository.List(ctx, cleaned)
}

func (s *MaterialService) Get(ctx context.Context, id int64) (models.Material, error) {
	material, err := s.repository.Get(ctx, id)
	return material, mapRepositoryError(err)
}

func (s *MaterialService) Update(ctx context.Context, id int64, input UpdateMaterialInput) (models.Material, error) {
	if !input.Name.Set && !input.Type.Set && !input.Brand.Set && !input.Color.Set &&
		!input.CostPerKG.Set && !input.StockKG.Set && !input.Active.Set {
		return models.Material{}, &ValidationError{Message: "Debes proporcionar al menos un campo"}
	}
	material, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.Material{}, mapRepositoryError(err)
	}
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.Material{}, requiredField("nombre")
		}
		material.Name = *input.Name.Value
	}
	if input.Type.Set {
		if input.Type.Value == nil {
			return models.Material{}, requiredField("tipo")
		}
		material.Type = *input.Type.Value
	}
	if input.Brand.Set {
		material.Brand = input.Brand.Value
	}
	if input.Color.Set {
		material.Color = input.Color.Value
	}
	if input.CostPerKG.Set {
		if input.CostPerKG.Value == nil {
			return models.Material{}, requiredField("costo_por_kg")
		}
		material.CostPerKG = *input.CostPerKG.Value
	}
	if input.StockKG.Set {
		if input.StockKG.Value == nil {
			return models.Material{}, requiredField("stock_kg")
		}
		material.StockKG = *input.StockKG.Value
	}
	if input.Active.Set {
		if input.Active.Value == nil {
			return models.Material{}, requiredField("activo")
		}
		material.Active = *input.Active.Value
	}
	cleanMaterial(&material)
	if err := validateMaterial(material); err != nil {
		return models.Material{}, err
	}
	updated, err := s.repository.Update(ctx, material)
	return updated, mapRepositoryError(err)
}

func cleanMaterial(material *models.Material) {
	material.Name = strings.TrimSpace(material.Name)
	material.Type = strings.TrimSpace(material.Type)
	material.Brand = cleanNullable(material.Brand)
	material.Color = cleanNullable(material.Color)
}

func validateMaterial(material models.Material) error {
	if material.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(material.Name) > 100 {
		return maxLength("nombre", 100)
	}
	if material.Type == "" {
		return requiredField("tipo")
	}
	if utf8.RuneCountInString(material.Type) > 50 {
		return maxLength("tipo", 50)
	}
	if material.Brand != nil && utf8.RuneCountInString(*material.Brand) > 100 {
		return maxLength("marca", 100)
	}
	if material.Color != nil && utf8.RuneCountInString(*material.Color) > 50 {
		return maxLength("color", 50)
	}
	if err := validateDecimal("costo_por_kg", material.CostPerKG, 9999999999.99); err != nil {
		return err
	}
	return validateDecimal("stock_kg", material.StockKG, 999999999.999)
}

func cleanFilter(value *string, field string, maximum int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(cleaned) > maximum {
		return nil, maxLength(field, maximum)
	}
	return &cleaned, nil
}

type rateRepository interface {
	MachineExists(context.Context, int64) (bool, error)
	LocationExists(context.Context, int64) (bool, error)
	GetMachineRate(context.Context, int64) (models.MachineRate, error)
	UpsertMachineRate(context.Context, models.MachineRate) (models.MachineRate, error)
	GetEnergyRate(context.Context, int64) (models.EnergyRate, error)
	UpsertEnergyRate(context.Context, models.EnergyRate) (models.EnergyRate, error)
}

type RateService struct {
	repository rateRepository
}

func NewRateService(repository rateRepository) *RateService {
	return &RateService{repository: repository}
}

func (s *RateService) GetMachineRate(ctx context.Context, machineID int64) (models.MachineRate, error) {
	if err := s.requireMachine(ctx, machineID); err != nil {
		return models.MachineRate{}, err
	}
	rate, err := s.repository.GetMachineRate(ctx, machineID)
	return rate, mapRepositoryError(err)
}

func (s *RateService) UpsertMachineRate(ctx context.Context, machineID int64, input MachineRateInput) (models.MachineRate, error) {
	if err := validateDecimal("costo_interno_hora", input.InternalCostHour, 9999999999.99); err != nil {
		return models.MachineRate{}, err
	}
	if err := validateDecimal("precio_venta_hora", input.SalePriceHour, 9999999999.99); err != nil {
		return models.MachineRate{}, err
	}
	if err := validateDecimal("costo_preparacion", input.PreparationCost, 9999999999.99); err != nil {
		return models.MachineRate{}, err
	}
	if err := s.requireMachine(ctx, machineID); err != nil {
		return models.MachineRate{}, err
	}
	rate, err := s.repository.UpsertMachineRate(ctx, models.MachineRate{
		MachineID: machineID, InternalCostHour: input.InternalCostHour,
		SalePriceHour: input.SalePriceHour, PreparationCost: input.PreparationCost,
	})
	return rate, mapMachineRepositoryError(err)
}

func (s *RateService) GetEnergyRate(ctx context.Context, locationID int64) (models.EnergyRate, error) {
	if err := s.requireLocation(ctx, locationID); err != nil {
		return models.EnergyRate{}, err
	}
	rate, err := s.repository.GetEnergyRate(ctx, locationID)
	return rate, mapRepositoryError(err)
}

func (s *RateService) UpsertEnergyRate(ctx context.Context, locationID int64, input EnergyRateInput) (models.EnergyRate, error) {
	if err := validateDecimal("costo_por_kwh", input.CostPerKWh, 99999999.9999); err != nil {
		return models.EnergyRate{}, err
	}
	if err := s.requireLocation(ctx, locationID); err != nil {
		return models.EnergyRate{}, err
	}
	rate, err := s.repository.UpsertEnergyRate(ctx, models.EnergyRate{LocationID: locationID, CostPerKWh: input.CostPerKWh})
	return rate, mapMachineRepositoryError(err)
}

func (s *RateService) requireMachine(ctx context.Context, id int64) error {
	exists, err := s.repository.MachineExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func (s *RateService) requireLocation(ctx context.Context, id int64) error {
	exists, err := s.repository.LocationExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func validateDecimal(field string, value, maximum float64) error {
	if value < 0 || value > maximum || math.IsNaN(value) || math.IsInf(value, 0) {
		return &ValidationError{Message: "El campo " + field + " debe estar entre 0 y su limite permitido"}
	}
	return nil
}
