package models

import "time"

type MachineState struct {
	ID          int64   `json:"id"`
	Code        string  `json:"codigo"`
	Name        string  `json:"nombre"`
	Description *string `json:"descripcion"`
}

type MachineStatePeriod struct {
	ID        int64      `json:"id"`
	MachineID int64      `json:"maquina_id"`
	WorkID    *int64     `json:"trabajo_id"`
	StateID   int64      `json:"estado_maquina_id"`
	StateCode string     `json:"estado_codigo"`
	StateName string     `json:"estado_nombre"`
	StartedAt time.Time  `json:"fecha_inicio"`
	EndedAt   *time.Time `json:"fecha_fin"`
	Notes     *string    `json:"notas"`
}

type MachineStateChange struct {
	Previous *MachineStatePeriod `json:"estado_anterior"`
	Current  MachineStatePeriod  `json:"estado_nuevo"`
}
