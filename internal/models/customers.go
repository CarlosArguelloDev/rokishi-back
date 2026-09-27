package models

import "time"

type Customer struct {
	ID            int64     `json:"id"`
	Type          string    `json:"tipo"`
	Name          string    `json:"nombre"`
	Email         *string   `json:"correo"`
	Phone         *string   `json:"telefono"`
	Notes         *string   `json:"notas"`
	Active        bool      `json:"activo"`
	CreationDate  time.Time `json:"fecha_creacion"`
	LastUpdatedAt time.Time `json:"fecha_actualizacion"`
}
