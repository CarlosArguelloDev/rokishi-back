package models

import "time"

type Machine struct {
	ID              int64     `json:"id"`
	LocationID      int64     `json:"locacion_id"`
	LocationCode    string    `json:"locacion_codigo"`
	LocationName    string    `json:"locacion_nombre"`
	MachineTypeID   int64     `json:"tipo_maquina_id"`
	MachineTypeName string    `json:"tipo_maquina_nombre"`
	Code            string    `json:"codigo"`
	Name            string    `json:"nombre"`
	Brand           *string   `json:"marca"`
	Model           *string   `json:"modelo"`
	SerialNumber    *string   `json:"numero_serie"`
	PowerWatts      *float64  `json:"potencia_watts"`
	PurchaseDate    *string   `json:"fecha_compra"`
	Active          bool      `json:"activa"`
	CreationDate    time.Time `json:"fecha_creacion"`
}
