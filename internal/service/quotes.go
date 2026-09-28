package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var decimalPattern = regexp.MustCompile(`^\d+(?:\.\d{1,3})?$`)
var percentagePattern = regexp.MustCompile(`^\d+(?:\.\d{1,2})?$`)

var quoteTransitions = map[string]map[string]bool{
	"BORRADOR": {"ENVIADA": true, "CANCELADA": true},
	"ENVIADA":  {"ACEPTADA": true, "RECHAZADA": true, "VENCIDA": true, "CANCELADA": true},
}

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

type CreateQuoteConceptInput struct {
	Description     *string
	MachineID       int64
	MaterialID      int64
	MaterialGrams   string
	DurationMinutes int64
	PieceCount      int64
}

type CreateQuoteInput struct {
	CustomerID     int64
	ExpirationDate *string
	Notes          *string
	Concepts       []CreateQuoteConceptInput
}

type QuoteFilters struct {
	CustomerID *int64
	StatusCode *string
}

type GenerateQuotePDFInput struct {
	ValidityDays       int
	ProductionTime     string
	DiscountPercentage string
	TaxPercentage      string
	Deposit            string
	Balance            string
	PaymentMethod      string
	Specifications     string
}

type quoteRepository interface {
	GetCalculationData(context.Context, int64, int64) (repository.QuoteData, error)
	ActiveCustomerExists(context.Context, int64) (bool, error)
	Create(context.Context, repository.CreateQuoteData) (models.Quote, error)
	List(context.Context, repository.QuoteFilters) ([]models.Quote, error)
	Get(context.Context, int64) (models.Quote, error)
	ListStatuses(context.Context) ([]models.QuoteStatus, error)
	ChangeStatus(context.Context, int64, int64, string) (models.Quote, error)
}

type quotePDFGenerator interface {
	Generate(models.Quote, models.QuotePDFOptions) ([]byte, error)
}

type QuoteService struct {
	repository quoteRepository
	pdf        quotePDFGenerator
}

func NewQuoteService(repository quoteRepository, generators ...quotePDFGenerator) *QuoteService {
	service := &QuoteService{repository: repository}
	if len(generators) > 0 {
		service.pdf = generators[0]
	}
	return service
}

func (s *QuoteService) Calculate(ctx context.Context, input CalculateQuoteInput) (models.QuoteCalculation, error) {
	calculation, _, err := s.calculate(ctx, input)
	return calculation, err
}

func (s *QuoteService) Create(ctx context.Context, input CreateQuoteInput) (models.Quote, error) {
	if input.CustomerID < 1 {
		return models.Quote{}, &ValidationError{Message: "El campo cliente_id debe ser un entero positivo"}
	}
	customerExists, err := s.repository.ActiveCustomerExists(ctx, input.CustomerID)
	if err != nil {
		return models.Quote{}, err
	}
	if !customerExists {
		return models.Quote{}, &ValidationError{Message: "El cliente no existe o esta inactivo"}
	}
	if len(input.Concepts) < 1 || len(input.Concepts) > 50 {
		return models.Quote{}, &ValidationError{Message: "La cotizacion debe contener entre 1 y 50 conceptos"}
	}
	notes := cleanNullable(input.Notes)
	if notes != nil && utf8.RuneCountInString(*notes) > 2000 {
		return models.Quote{}, maxLength("notas", 2000)
	}
	expirationDate, err := parseQuoteExpiration(input.ExpirationDate)
	if err != nil {
		return models.Quote{}, err
	}

	snapshots := make([]repository.QuoteConceptSnapshot, 0, len(input.Concepts))
	var totalCost, totalSuggestedPrice models.Money
	for index, concept := range input.Concepts {
		description := cleanNullable(concept.Description)
		if description != nil && utf8.RuneCountInString(*description) > 200 {
			return models.Quote{}, &ValidationError{Message: "La descripcion del concepto no puede exceder 200 caracteres"}
		}
		calculation, rates, err := s.calculate(ctx, CalculateQuoteInput{
			MachineID: concept.MachineID, MaterialID: concept.MaterialID,
			MaterialGrams: concept.MaterialGrams, DurationMinutes: concept.DurationMinutes,
			PieceCount: concept.PieceCount,
		})
		if err != nil {
			var validationError *ValidationError
			var configurationError *MissingConfigurationError
			switch {
			case errors.As(err, &validationError):
				return models.Quote{}, &ValidationError{Message: "Concepto " + strconv.Itoa(index+1) + ": " + validationError.Message}
			case errors.As(err, &configurationError):
				return models.Quote{}, &MissingConfigurationError{Message: "Concepto " + strconv.Itoa(index+1) + ": " + configurationError.Message}
			default:
				return models.Quote{}, err
			}
		}
		if int64(totalCost) > math.MaxInt64-int64(calculation.Subtotal) ||
			int64(totalSuggestedPrice) > math.MaxInt64-int64(calculation.SuggestedPrice) {
			return models.Quote{}, &ValidationError{Message: "El total de la cotizacion excede el limite permitido"}
		}
		totalCost += calculation.Subtotal
		totalSuggestedPrice += calculation.SuggestedPrice
		snapshots = append(snapshots, repository.QuoteConceptSnapshot{
			Description: description, Calculation: calculation, Rates: rates,
		})
	}

	created, err := s.repository.Create(ctx, repository.CreateQuoteData{
		CustomerID: input.CustomerID, ExpirationDate: expirationDate, Notes: notes,
		TotalCost: totalCost, TotalSuggestedPrice: totalSuggestedPrice, Concepts: snapshots,
	})
	if errors.Is(err, repository.ErrReferenceMissing) {
		return models.Quote{}, &ValidationError{Message: "El cliente, la maquina o el material ya no existe"}
	}
	return created, mapRepositoryError(err)
}

func (s *QuoteService) List(ctx context.Context, filters QuoteFilters) ([]models.Quote, error) {
	if filters.CustomerID != nil && *filters.CustomerID < 1 {
		return nil, &ValidationError{Message: "El filtro cliente_id debe ser un entero positivo"}
	}
	if filters.StatusCode != nil {
		code := strings.ToUpper(strings.TrimSpace(*filters.StatusCode))
		if !knownQuoteStatus(code) {
			return nil, &ValidationError{Message: "El filtro estado no es valido"}
		}
		filters.StatusCode = &code
	}
	return s.repository.List(ctx, repository.QuoteFilters{
		CustomerID: filters.CustomerID, StatusCode: filters.StatusCode,
	})
}

func (s *QuoteService) Get(ctx context.Context, id int64) (models.Quote, error) {
	if id < 1 {
		return models.Quote{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	quote, err := s.repository.Get(ctx, id)
	return quote, mapRepositoryError(err)
}

func (s *QuoteService) GeneratePDF(ctx context.Context, id int64, input GenerateQuotePDFInput) (models.QuotePDF, error) {
	if id < 1 {
		return models.QuotePDF{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	quote, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.QuotePDF{}, mapRepositoryError(err)
	}

	validityDays := input.ValidityDays
	if validityDays == 0 {
		validityDays = defaultValidityDays(quote)
	}
	if validityDays < 1 || validityDays > 365 {
		return models.QuotePDF{}, &ValidationError{Message: "La vigencia debe estar entre 1 y 365 dias"}
	}
	discountBasisPoints, err := parsePercentage(input.DiscountPercentage, "descuento_porcentaje", "0")
	if err != nil {
		return models.QuotePDF{}, err
	}
	taxBasisPoints, err := parsePercentage(input.TaxPercentage, "iva_porcentaje", "16")
	if err != nil {
		return models.QuotePDF{}, err
	}

	productionTime, err := quotePDFText(input.ProductionTime, "tiempo_produccion", 120, "Por acordar")
	if err != nil {
		return models.QuotePDF{}, err
	}
	deposit, err := quotePDFText(input.Deposit, "anticipo", 80, "Por acordar")
	if err != nil {
		return models.QuotePDF{}, err
	}
	balance, err := quotePDFText(input.Balance, "saldo", 80, "Por acordar")
	if err != nil {
		return models.QuotePDF{}, err
	}
	paymentMethod, err := quotePDFText(input.PaymentMethod, "forma_pago", 80, "Por acordar")
	if err != nil {
		return models.QuotePDF{}, err
	}
	specifications, err := quotePDFText(input.Specifications, "especificaciones", 1000, "")
	if err != nil {
		return models.QuotePDF{}, err
	}

	discountAmount := percentageOfMoney(quote.TotalSuggestedPrice, discountBasisPoints)
	taxBase := quote.TotalSuggestedPrice - discountAmount
	taxAmount := percentageOfMoney(taxBase, taxBasisPoints)
	options := models.QuotePDFOptions{
		ValidityDays: validityDays, ProductionTime: productionTime,
		Deposit: deposit, Balance: balance, PaymentMethod: paymentMethod,
		Specifications: specifications, DiscountBasisPoints: discountBasisPoints,
		TaxBasisPoints: taxBasisPoints, DiscountAmount: discountAmount,
		TaxAmount: taxAmount, TotalAfterDiscountTax: taxBase + taxAmount,
	}
	if s.pdf == nil {
		return models.QuotePDF{}, errors.New("generador de PDF no configurado")
	}
	content, err := s.pdf.Generate(quote, options)
	if err != nil {
		return models.QuotePDF{}, err
	}
	return models.QuotePDF{
		Filename: "cotizacion-COT-" + fmt.Sprintf("%06d", quote.ID) + ".pdf",
		Content:  content,
	}, nil
}

func (s *QuoteService) ListStatuses(ctx context.Context) ([]models.QuoteStatus, error) {
	return s.repository.ListStatuses(ctx)
}

func (s *QuoteService) ChangeStatus(ctx context.Context, id int64, statusCode string) (models.Quote, error) {
	if id < 1 {
		return models.Quote{}, &ValidationError{Message: "El identificador debe ser un entero positivo"}
	}
	target := strings.ToUpper(strings.TrimSpace(statusCode))
	if !knownQuoteStatus(target) {
		return models.Quote{}, &ValidationError{Message: "El estado de cotizacion no es valido"}
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return models.Quote{}, mapRepositoryError(err)
	}
	if !quoteTransitions[current.StatusCode][target] {
		return models.Quote{}, &ValidationError{Message: "La transicion de " + current.StatusCode + " a " + target + " no esta permitida"}
	}
	updated, err := s.repository.ChangeStatus(ctx, id, current.StatusID, target)
	if errors.Is(err, repository.ErrConflict) {
		return models.Quote{}, ErrConflict
	}
	return updated, mapRepositoryError(err)
}

func (s *QuoteService) calculate(ctx context.Context, input CalculateQuoteInput) (models.QuoteCalculation, repository.QuoteData, error) {
	if input.MachineID < 1 {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "El campo maquina_id debe ser un entero positivo"}
	}
	if input.MaterialID < 1 {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "El campo material_id debe ser un entero positivo"}
	}
	grams, gramsText, err := parsePositiveDecimal(input.MaterialGrams, "cantidad_material_gramos")
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	if input.DurationMinutes < 1 || input.DurationMinutes > 5256000 {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "El campo duracion_minutos debe estar entre 1 y 5256000"}
	}
	if input.PieceCount < 1 || input.PieceCount > 1000000 {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "El campo cantidad_piezas debe estar entre 1 y 1000000"}
	}

	data, err := s.repository.GetCalculationData(ctx, input.MachineID, input.MaterialID)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, mapRepositoryError(err)
	}
	if !data.MachineActive {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "La maquina seleccionada esta inactiva"}
	}
	if !data.MaterialActive {
		return models.QuoteCalculation{}, repository.QuoteData{}, &ValidationError{Message: "El material seleccionado esta inactivo"}
	}
	if data.PowerWatts == nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, missingConfiguration("La maquina no tiene potencia configurada")
	}
	if data.InternalCostHour == nil || data.SalePriceHour == nil || data.PreparationCost == nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, missingConfiguration("La maquina no tiene tarifa configurada")
	}
	if data.EnergyCostPerKWh == nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, missingConfiguration("La locacion de la maquina no tiene tarifa electrica configurada")
	}

	materialCostPerKG, err := decimalRat(data.MaterialCostPerKG)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	internalCostHour, err := decimalRat(*data.InternalCostHour)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	salePriceHour, err := decimalRat(*data.SalePriceHour)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	preparationCost, err := decimalRat(*data.PreparationCost)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	powerWatts, err := decimalRat(*data.PowerWatts)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
	}
	energyCostPerKWh, err := decimalRat(*data.EnergyCostPerKWh)
	if err != nil {
		return models.QuoteCalculation{}, repository.QuoteData{}, err
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
	}, data, nil
}

func parseQuoteExpiration(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, &ValidationError{Message: "El campo fecha_vencimiento debe tener formato RFC3339"}
	}
	if !parsed.After(time.Now()) {
		return nil, &ValidationError{Message: "La fecha de vencimiento debe ser futura"}
	}
	return &parsed, nil
}

func knownQuoteStatus(code string) bool {
	if code == "BORRADOR" {
		return true
	}
	_, hasTransitions := quoteTransitions[code]
	if hasTransitions {
		return true
	}
	for _, targets := range quoteTransitions {
		if targets[code] {
			return true
		}
	}
	return false
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

func defaultValidityDays(quote models.Quote) int {
	if quote.ExpirationDate == nil {
		return 15
	}
	days := int(math.Ceil(quote.ExpirationDate.Sub(quote.CreationDate).Hours() / 24))
	if days < 1 || days > 365 {
		return 15
	}
	return days
}

func parsePercentage(value, field, fallback string) (int64, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		cleaned = fallback
	}
	if !percentagePattern.MatchString(cleaned) {
		return 0, &ValidationError{Message: "El campo " + field + " debe ser un porcentaje entre 0 y 100 con hasta 2 decimales"}
	}
	parsed, ok := new(big.Rat).SetString(cleaned)
	if !ok || parsed.Sign() < 0 || parsed.Cmp(big.NewRat(100, 1)) > 0 {
		return 0, &ValidationError{Message: "El campo " + field + " debe estar entre 0 y 100"}
	}
	scaled := new(big.Rat).Mul(parsed, big.NewRat(100, 1))
	return new(big.Int).Quo(scaled.Num(), scaled.Denom()).Int64(), nil
}

func quotePDFText(value, field string, limit int, fallback string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return fallback, nil
	}
	if utf8.RuneCountInString(cleaned) > limit {
		return "", maxLength(field, limit)
	}
	return cleaned, nil
}

func percentageOfMoney(value models.Money, basisPoints int64) models.Money {
	product := new(big.Int).Mul(big.NewInt(int64(value)), big.NewInt(basisPoints))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, big.NewInt(10000), remainder)
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(big.NewInt(10000)) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return models.Money(quotient.Int64())
}

func missingConfiguration(message string) error {
	return &MissingConfigurationError{Message: message}
}
