package models

import (
	"encoding/json"
	"time"
)

type Order struct {
	ID             int64     `json:"id"`
	QuoteID        *int64    `json:"cotizacion_id"`
	CustomerID     int64     `json:"cliente_id"`
	CustomerName   string    `json:"cliente_nombre"`
	CustomerType   string    `json:"cliente_tipo"`
	Origin         string    `json:"origen"`
	SalesPlatform  *string   `json:"plataforma_venta"`
	Notes          *string   `json:"notas"`
	Status         string    `json:"estado"`
	WorkCount      int64     `json:"cantidad_trabajos"`
	CompletedCount int64     `json:"trabajos_completados"`
	CreationDate   time.Time `json:"fecha_creacion"`
	LastUpdatedAt  time.Time `json:"fecha_actualizacion"`
	Works          []Work    `json:"trabajos,omitempty"`
}

type Work struct {
	ID                    int64         `json:"id"`
	OrderID               int64         `json:"pedido_id"`
	QuoteConceptID        *int64        `json:"concepto_cotizacion_id"`
	RequiredMachineTypeID int64         `json:"tipo_maquina_id_requerido"`
	RequiredMachineType   string        `json:"tipo_maquina_requerido"`
	MachineID             *int64        `json:"maquina_id"`
	MachineCode           *string       `json:"maquina_codigo"`
	MachineName           *string       `json:"maquina_nombre"`
	MaterialID            int64         `json:"material_id"`
	MaterialName          string        `json:"material_nombre"`
	Description           *string       `json:"descripcion"`
	PieceCount            int64         `json:"cantidad_piezas"`
	EstimatedMinutes      int64         `json:"duracion_estimada_minutos"`
	EstimatedMaterial     json.Number   `json:"material_estimado_gramos"`
	Status                string        `json:"estado"`
	CreationDate          time.Time     `json:"fecha_creacion"`
	LastUpdatedAt         time.Time     `json:"fecha_actualizacion"`
	Attempts              []WorkAttempt `json:"intentos,omitempty"`
}

type WorkAttempt struct {
	ID               int64        `json:"id"`
	WorkID           int64        `json:"trabajo_id"`
	AttemptNumber    int64        `json:"numero_intento"`
	MachineID        int64        `json:"maquina_id"`
	MachineCode      string       `json:"maquina_codigo"`
	MachineName      string       `json:"maquina_nombre"`
	StartedAt        time.Time    `json:"fecha_inicio"`
	EndedAt          *time.Time   `json:"fecha_fin"`
	ActualSeconds    *int64       `json:"duracion_real_segundos"`
	Result           *string      `json:"resultado"`
	ConsumedMaterial *json.Number `json:"material_consumido_gramos"`
	Waste            *json.Number `json:"desperdicio_gramos"`
	Notes            *string      `json:"notas"`
}
