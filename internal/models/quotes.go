package models

import (
	"encoding/json"
	"strconv"
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
