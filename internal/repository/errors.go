package repository

import "errors"

var (
	ErrNotFound           = errors.New("registro no encontrado")
	ErrConflict           = errors.New("registro duplicado")
	ErrReferenceMissing   = errors.New("referencia no encontrada")
	ErrStateUnchanged     = errors.New("la maquina ya tiene ese estado")
	ErrInvalidStateTime   = errors.New("fecha de cambio invalida")
	ErrInvalidOperation   = errors.New("operacion no permitida en el estado actual")
	ErrMachineUnavailable = errors.New("maquina no disponible")
	ErrSetupComplete      = errors.New("la configuracion inicial ya fue completada")
)
