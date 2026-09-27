package repository

import (
	"context"

	"rokishi-back/internal/models"
)

type MachineFilters struct {
	LocationID    *int64
	MachineTypeID *int64
	Active        *bool
	Query         *string
}

type MachineRepository struct {
	db DB
}

func NewMachineRepository(db DB) *MachineRepository {
	return &MachineRepository{db: db}
}

func (r *MachineRepository) Create(ctx context.Context, machine models.Machine) (models.Machine, error) {
	row := r.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO maquinas (
				locacion_id, tipo_maquina_id, codigo, nombre, marca, modelo,
				numero_serie, potencia_watts, fecha_compra, activa
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING *
		)
		SELECT i.id, i.locacion_id, l.codigo, l.nombre, i.tipo_maquina_id, tm.nombre,
			i.codigo, i.nombre, i.marca, i.modelo, i.numero_serie, i.potencia_watts,
			i.fecha_compra::text, i.activa, i.fecha_creacion
		FROM inserted i
		JOIN locaciones l ON l.id = i.locacion_id
		JOIN tipos_maquina tm ON tm.id = i.tipo_maquina_id`,
		machine.LocationID, machine.MachineTypeID, machine.Code, machine.Name, machine.Brand,
		machine.Model, machine.SerialNumber, machine.PowerWatts, machine.PurchaseDate, machine.Active)
	created, err := scanMachine(row)
	return created, translateError(err)
}

func (r *MachineRepository) List(ctx context.Context, filters MachineFilters) ([]models.Machine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.locacion_id, l.codigo, l.nombre, m.tipo_maquina_id, tm.nombre,
			m.codigo, m.nombre, m.marca, m.modelo, m.numero_serie, m.potencia_watts,
			m.fecha_compra::text, m.activa, m.fecha_creacion
		FROM maquinas m
		JOIN locaciones l ON l.id = m.locacion_id
		JOIN tipos_maquina tm ON tm.id = m.tipo_maquina_id
		WHERE ($1::bigint IS NULL OR m.locacion_id = $1)
			AND ($2::bigint IS NULL OR m.tipo_maquina_id = $2)
			AND ($3::boolean IS NULL OR m.activa = $3)
			AND ($4::text IS NULL OR m.codigo ILIKE '%' || $4 || '%' OR m.nombre ILIKE '%' || $4 || '%')
		ORDER BY m.nombre, m.id`, filters.LocationID, filters.MachineTypeID, filters.Active, filters.Query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	machines := make([]models.Machine, 0)
	for rows.Next() {
		machine, err := scanMachine(rows)
		if err != nil {
			return nil, err
		}
		machines = append(machines, machine)
	}
	return machines, rows.Err()
}

func (r *MachineRepository) Get(ctx context.Context, id int64) (models.Machine, error) {
	row := r.db.QueryRow(ctx, `
		SELECT m.id, m.locacion_id, l.codigo, l.nombre, m.tipo_maquina_id, tm.nombre,
			m.codigo, m.nombre, m.marca, m.modelo, m.numero_serie, m.potencia_watts,
			m.fecha_compra::text, m.activa, m.fecha_creacion
		FROM maquinas m
		JOIN locaciones l ON l.id = m.locacion_id
		JOIN tipos_maquina tm ON tm.id = m.tipo_maquina_id
		WHERE m.id = $1`, id)
	machine, err := scanMachine(row)
	return machine, translateError(err)
}

func (r *MachineRepository) Update(ctx context.Context, machine models.Machine) (models.Machine, error) {
	row := r.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE maquinas
			SET locacion_id = $2, tipo_maquina_id = $3, codigo = $4, nombre = $5,
				marca = $6, modelo = $7, numero_serie = $8, potencia_watts = $9,
				fecha_compra = $10, activa = $11
			WHERE id = $1
			RETURNING *
		)
		SELECT u.id, u.locacion_id, l.codigo, l.nombre, u.tipo_maquina_id, tm.nombre,
			u.codigo, u.nombre, u.marca, u.modelo, u.numero_serie, u.potencia_watts,
			u.fecha_compra::text, u.activa, u.fecha_creacion
		FROM updated u
		JOIN locaciones l ON l.id = u.locacion_id
		JOIN tipos_maquina tm ON tm.id = u.tipo_maquina_id`,
		machine.ID, machine.LocationID, machine.MachineTypeID, machine.Code, machine.Name,
		machine.Brand, machine.Model, machine.SerialNumber, machine.PowerWatts,
		machine.PurchaseDate, machine.Active)
	updated, err := scanMachine(row)
	return updated, translateError(err)
}

func (r *MachineRepository) LocationExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM locaciones WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (r *MachineRepository) MachineTypeExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tipos_maquina WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func scanMachine(row scanner) (models.Machine, error) {
	var machine models.Machine
	err := row.Scan(
		&machine.ID,
		&machine.LocationID,
		&machine.LocationCode,
		&machine.LocationName,
		&machine.MachineTypeID,
		&machine.MachineTypeName,
		&machine.Code,
		&machine.Name,
		&machine.Brand,
		&machine.Model,
		&machine.SerialNumber,
		&machine.PowerWatts,
		&machine.PurchaseDate,
		&machine.Active,
		&machine.CreationDate,
	)
	return machine, err
}
