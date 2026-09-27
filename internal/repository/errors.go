package repository

import "errors"

var (
	ErrNotFound         = errors.New("registro no encontrado")
	ErrConflict         = errors.New("registro duplicado")
	ErrReferenceMissing = errors.New("referencia no encontrada")
)
