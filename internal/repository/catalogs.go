package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"rokishi-back/internal/models"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type LocationRepository struct {
	db DB
}

func NewLocationRepository(db DB) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) Create(ctx context.Context, location models.Location) (models.Location, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO locaciones (codigo, nombre, direccion, ciudad, estado, zona_horaria, activa)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, codigo, nombre, direccion, ciudad, estado, zona_horaria, activa, fecha_creacion`,
		location.Code, location.Name, location.Address, location.City, location.State, location.Timezone, location.Active)
	created, err := scanLocation(row)
	return created, translateError(err)
}

func (r *LocationRepository) List(ctx context.Context) ([]models.Location, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, codigo, nombre, direccion, ciudad, estado, zona_horaria, activa, fecha_creacion
		FROM locaciones
		ORDER BY nombre, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := make([]models.Location, 0)
	for rows.Next() {
		location, err := scanLocation(rows)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func (r *LocationRepository) Get(ctx context.Context, id int64) (models.Location, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, codigo, nombre, direccion, ciudad, estado, zona_horaria, activa, fecha_creacion
		FROM locaciones
		WHERE id = $1`, id)
	location, err := scanLocation(row)
	return location, translateError(err)
}

func (r *LocationRepository) Update(ctx context.Context, location models.Location) (models.Location, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE locaciones
		SET codigo = $2, nombre = $3, direccion = $4, ciudad = $5, estado = $6,
			zona_horaria = $7, activa = $8
		WHERE id = $1
		RETURNING id, codigo, nombre, direccion, ciudad, estado, zona_horaria, activa, fecha_creacion`,
		location.ID, location.Code, location.Name, location.Address, location.City, location.State,
		location.Timezone, location.Active)
	updated, err := scanLocation(row)
	return updated, translateError(err)
}

type MachineTypeRepository struct {
	db DB
}

func NewMachineTypeRepository(db DB) *MachineTypeRepository {
	return &MachineTypeRepository{db: db}
}

func (r *MachineTypeRepository) Create(ctx context.Context, machineType models.MachineType) (models.MachineType, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO tipos_maquina (nombre, descripcion)
		VALUES ($1, $2)
		RETURNING id, nombre, descripcion`, machineType.Name, machineType.Description)
	created, err := scanMachineType(row)
	return created, translateError(err)
}

func (r *MachineTypeRepository) List(ctx context.Context) ([]models.MachineType, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, nombre, descripcion
		FROM tipos_maquina
		ORDER BY nombre, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	machineTypes := make([]models.MachineType, 0)
	for rows.Next() {
		machineType, err := scanMachineType(rows)
		if err != nil {
			return nil, err
		}
		machineTypes = append(machineTypes, machineType)
	}
	return machineTypes, rows.Err()
}

func (r *MachineTypeRepository) Get(ctx context.Context, id int64) (models.MachineType, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, nombre, descripcion
		FROM tipos_maquina
		WHERE id = $1`, id)
	machineType, err := scanMachineType(row)
	return machineType, translateError(err)
}

func (r *MachineTypeRepository) Update(ctx context.Context, machineType models.MachineType) (models.MachineType, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE tipos_maquina
		SET nombre = $2, descripcion = $3
		WHERE id = $1
		RETURNING id, nombre, descripcion`, machineType.ID, machineType.Name, machineType.Description)
	updated, err := scanMachineType(row)
	return updated, translateError(err)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanLocation(row scanner) (models.Location, error) {
	var location models.Location
	err := row.Scan(
		&location.ID,
		&location.Code,
		&location.Name,
		&location.Address,
		&location.City,
		&location.State,
		&location.Timezone,
		&location.Active,
		&location.CreationDate,
	)
	return location, err
}

func scanMachineType(row scanner) (models.MachineType, error) {
	var machineType models.MachineType
	err := row.Scan(&machineType.ID, &machineType.Name, &machineType.Description)
	return machineType, err
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
