package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"regexp"
	"strings"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var decimalPattern = regexp.MustCompile(`^\d+(?:\.\d{1,3})?$`)

type MissingConfigurationError struct {
	Message string
}

func (e *MissingConfigurationError) Error() string { return e.Message }

type CalculateQuoteInput struct {
	MachineID       int64
	MaterialID      int64
	MaterialGrams   string
	DurationMinutes int64
	PieceCount      int64
}

type quoteRepository interface {
	GetCalculationData(context.Context, int64, int64) (repository.QuoteData, error)
}

type QuoteService struct {
	repository quoteRepository
}

func NewQuoteService(repository quoteRepository) *QuoteService {
	return &QuoteService{repository: repository}
}

func (s *QuoteService) Calculate(ctx context.Context, input CalculateQuoteInput) (models.QuoteCalculation, error) {
	if input.MachineID < 1 {
		return models.QuoteCalculation{}, &ValidationError{Message: "El campo maquina_id debe ser un entero positivo"}
	}
	if input.MaterialID < 1 {
		return models.QuoteCalculation{}, &ValidationError{Message: "El campo material_id debe ser un entero positivo"}
	}
	grams, gramsText, err := parsePositiveDecimal(input.MaterialGrams, "cantidad_material_gramos")
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	if input.DurationMinutes < 1 || input.DurationMinutes > 5256000 {
		return models.QuoteCalculation{}, &ValidationError{Message: "El campo duracion_minutos debe estar entre 1 y 5256000"}
	}
	if input.PieceCount < 1 || input.PieceCount > 1000000 {
		return models.QuoteCalculation{}, &ValidationError{Message: "El campo cantidad_piezas debe estar entre 1 y 1000000"}
	}

	data, err := s.repository.GetCalculationData(ctx, input.MachineID, input.MaterialID)
	if err != nil {
		return models.QuoteCalculation{}, mapRepositoryError(err)
	}
	if !data.MachineActive {
		return models.QuoteCalculation{}, &ValidationError{Message: "La maquina seleccionada esta inactiva"}
	}
	if !data.MaterialActive {
		return models.QuoteCalculation{}, &ValidationError{Message: "El material seleccionado esta inactivo"}
	}
	if data.PowerWatts == nil {
		return models.QuoteCalculation{}, missingConfiguration("La maquina no tiene potencia configurada")
	}
	if data.InternalCostHour == nil || data.SalePriceHour == nil || data.PreparationCost == nil {
		return models.QuoteCalculation{}, missingConfiguration("La maquina no tiene tarifa configurada")
	}
	if data.EnergyCostPerKWh == nil {
		return models.QuoteCalculation{}, missingConfiguration("La locacion de la maquina no tiene tarifa electrica configurada")
	}

	materialCostPerKG, err := decimalRat(data.MaterialCostPerKG)
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	internalCostHour, err := decimalRat(*data.InternalCostHour)
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	salePriceHour, err := decimalRat(*data.SalePriceHour)
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	preparationCost, err := decimalRat(*data.PreparationCost)
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	powerWatts, err := decimalRat(*data.PowerWatts)
	if err != nil {
		return models.QuoteCalculation{}, err
	}
	energyCostPerKWh, err := decimalRat(*data.EnergyCostPerKWh)
	if err != nil {
		return models.QuoteCalculation{}, err
	}

	duration := big.NewRat(input.DurationMinutes, 60)
	materialCost := moneyFromRat(new(big.Rat).Mul(materialCostPerKG, new(big.Rat).Quo(grams, big.NewRat(1000, 1))))
	machineCost := moneyFromRat(new(big.Rat).Mul(internalCostHour, duration))
	electricity := new(big.Rat).Mul(powerWatts, duration)
	electricity.Quo(electricity, big.NewRat(1000, 1))
	electricity.Mul(electricity, energyCostPerKWh)
	electricityCost := moneyFromRat(electricity)
	preparation := moneyFromRat(preparationCost)
	saleMachineCost := moneyFromRat(new(big.Rat).Mul(salePriceHour, duration))
	subtotal := materialCost + machineCost + electricityCost + preparation
	suggested := materialCost + saleMachineCost + electricityCost + preparation

	return models.QuoteCalculation{
		MachineID: input.MachineID, MaterialID: input.MaterialID, MaterialGrams: json.Number(gramsText),
		DurationMinutes: input.DurationMinutes, PieceCount: input.PieceCount,
		MaterialCost: materialCost, MachineCost: machineCost, ElectricityCost: electricityCost,
		PreparationCost: preparation, Subtotal: subtotal, SuggestedPrice: suggested,
		SuggestedPricePerPiece: divideMoney(suggested, input.PieceCount),
	}, nil
}

func parsePositiveDecimal(value, field string) (*big.Rat, string, error) {
	cleaned := strings.TrimSpace(value)
	if !decimalPattern.MatchString(cleaned) {
		return nil, "", &ValidationError{Message: "El campo " + field + " debe ser un numero positivo con hasta 3 decimales"}
	}
	parsed, ok := new(big.Rat).SetString(cleaned)
	if !ok || parsed.Sign() <= 0 {
		return nil, "", &ValidationError{Message: "El campo " + field + " debe ser mayor a cero"}
	}
	if parsed.Cmp(big.NewRat(999999999999, 1000)) > 0 {
		return nil, "", &ValidationError{Message: "El campo " + field + " excede el limite permitido"}
	}
	return parsed, cleaned, nil
}

func decimalRat(value string) (*big.Rat, error) {
	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, errors.New("valor decimal invalido en la configuracion")
	}
	return parsed, nil
}

func moneyFromRat(value *big.Rat) models.Money {
	scaled := new(big.Rat).Mul(value, big.NewRat(100, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(scaled.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return models.Money(quotient.Int64())
}

func divideMoney(value models.Money, divisor int64) models.Money {
	quotient, remainder := int64(value)/divisor, int64(value)%divisor
	if remainder*2 >= divisor {
		quotient++
	}
	return models.Money(quotient)
}

func missingConfiguration(message string) error {
	return &MissingConfigurationError{Message: message}
}
