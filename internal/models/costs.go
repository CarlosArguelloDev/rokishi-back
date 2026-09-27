package models

import "time"

type Material struct {
	ID            int64     `json:"id"`
	Name          string    `json:"nombre"`
	Type          string    `json:"tipo"`
	Brand         *string   `json:"marca"`
	Color         *string   `json:"color"`
	CostPerKG     float64   `json:"costo_por_kg"`
	StockKG       float64   `json:"stock_kg"`
	Active        bool      `json:"activo"`
	LastUpdatedAt time.Time `json:"fecha_actualizacion"`
}

type MachineRate struct {
	ID               int64     `json:"id"`
	MachineID        int64     `json:"maquina_id"`
	InternalCostHour float64   `json:"costo_interno_hora"`
	SalePriceHour    float64   `json:"precio_venta_hora"`
	PreparationCost  float64   `json:"costo_preparacion"`
	LastUpdatedAt    time.Time `json:"fecha_actualizacion"`
}

type EnergyRate struct {
	ID            int64     `json:"id"`
	LocationID    int64     `json:"locacion_id"`
	CostPerKWh    float64   `json:"costo_por_kwh"`
	LastUpdatedAt time.Time `json:"fecha_actualizacion"`
}
