package models

import "time"

type Location struct {
	ID           int64     `json:"id"`
	Code         string    `json:"codigo"`
	Name         string    `json:"nombre"`
	Address      *string   `json:"direccion"`
	City         *string   `json:"ciudad"`
	State        *string   `json:"estado"`
	Timezone     string    `json:"zona_horaria"`
	Active       bool      `json:"activa"`
	CreationDate time.Time `json:"fecha_creacion"`
}

type MachineType struct {
	ID          int64   `json:"id"`
	Name        string  `json:"nombre"`
	Description *string `json:"descripcion"`
}
