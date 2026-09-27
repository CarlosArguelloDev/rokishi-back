package models

import (
	"encoding/json"
	"strconv"
	"time"
)

type Money int64

func (m Money) MarshalJSON() ([]byte, error) {
	cents := int64(m)
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return []byte(sign + strconv.FormatInt(cents/100, 10) + "." + twoDigits(cents%100)), nil
}

func twoDigits(value int64) string {
	if value < 10 {
		return "0" + strconv.FormatInt(value, 10)
	}
	return strconv.FormatInt(value, 10)
}

type QuoteCalculation struct {
	MachineID              int64       `json:"maquina_id"`
	MaterialID             int64       `json:"material_id"`
	MaterialGrams          json.Number `json:"cantidad_material_gramos"`
	DurationMinutes        int64       `json:"duracion_minutos"`
	PieceCount             int64       `json:"cantidad_piezas"`
	MaterialCost           Money       `json:"costo_material"`
	MachineCost            Money       `json:"costo_maquina"`
	ElectricityCost        Money       `json:"costo_electrico"`
	PreparationCost        Money       `json:"costo_preparacion"`
	Subtotal               Money       `json:"subtotal"`
	SuggestedPrice         Money       `json:"precio_sugerido"`
	SuggestedPricePerPiece Money       `json:"precio_sugerido_por_pieza"`
}

type QuoteStatus struct {
	ID          int64   `json:"id"`
	Code        string  `json:"codigo"`
	Name        string  `json:"nombre"`
	Description *string `json:"descripcion"`
}

type QuoteConcept struct {
	ID                     int64       `json:"id"`
	QuoteID                int64       `json:"cotizacion_id"`
	MachineID              int64       `json:"maquina_id"`
	MaterialID             int64       `json:"material_id"`
	MachineCode            string      `json:"maquina_codigo"`
	MachineName            string      `json:"maquina_nombre"`
	MaterialName           string      `json:"material_nombre"`
	Description            *string     `json:"descripcion"`
	MaterialGrams          json.Number `json:"cantidad_material_gramos"`
	DurationMinutes        int64       `json:"duracion_minutos"`
	PieceCount             int64       `json:"cantidad_piezas"`
	MaterialCostPerKG      json.Number `json:"costo_material_por_kg"`
	InternalCostHour       json.Number `json:"costo_interno_hora"`
	SalePriceHour          json.Number `json:"precio_venta_hora"`
	RatePreparationCost    json.Number `json:"tarifa_costo_preparacion"`
	PowerWatts             json.Number `json:"potencia_watts"`
	EnergyCostPerKWh       json.Number `json:"costo_por_kwh"`
	MaterialCost           Money       `json:"costo_material"`
	MachineCost            Money       `json:"costo_maquina"`
	ElectricityCost        Money       `json:"costo_electrico"`
	PreparationCost        Money       `json:"costo_preparacion"`
	Subtotal               Money       `json:"subtotal"`
	SuggestedPrice         Money       `json:"precio_sugerido"`
	SuggestedPricePerPiece Money       `json:"precio_sugerido_por_pieza"`
	CreationDate           time.Time   `json:"fecha_creacion"`
}

type Quote struct {
	ID                  int64          `json:"id"`
	CustomerID          int64          `json:"cliente_id"`
	CustomerName        string         `json:"cliente_nombre"`
	StatusID            int64          `json:"estado_cotizacion_id"`
	StatusCode          string         `json:"estado_codigo"`
	StatusName          string         `json:"estado_nombre"`
	ExpirationDate      *time.Time     `json:"fecha_vencimiento"`
	Notes               *string        `json:"notas"`
	TotalCost           Money          `json:"costo_total"`
	TotalSuggestedPrice Money          `json:"precio_sugerido_total"`
	CreationDate        time.Time      `json:"fecha_creacion"`
	LastUpdatedAt       time.Time      `json:"fecha_actualizacion"`
	Concepts            []QuoteConcept `json:"conceptos,omitempty"`
}
