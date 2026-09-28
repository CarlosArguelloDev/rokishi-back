package models

import "time"

const (
	RoleAdmin    = "ADMIN"
	RoleOperator = "OPERADOR"
)

type User struct {
	ID            int64     `json:"id"`
	Name          string    `json:"nombre"`
	Email         string    `json:"correo"`
	Role          string    `json:"rol"`
	Active        bool      `json:"activo"`
	CreationDate  time.Time `json:"fecha_creacion"`
	LastUpdatedAt time.Time `json:"fecha_actualizacion"`
}

type AuditEntry struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"usuario_id"`
	UserName   string    `json:"usuario_nombre"`
	Action     string    `json:"accion"`
	Resource   string    `json:"recurso"`
	HTTPStatus int       `json:"estado_http"`
	IPAddress  *string   `json:"direccion_ip"`
	UserAgent  *string   `json:"agente_usuario"`
	Date       time.Time `json:"fecha"`
}
