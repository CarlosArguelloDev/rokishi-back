package repository

import (
	"context"

	"rokishi-back/internal/models"
)

type MaterialFilters struct {
	Type   *string
	Brand  *string
	Color  *string
	Active *bool
}

type MaterialRepository struct {
	db DB
}

func NewMaterialRepository(db DB) *MaterialRepository {
	return &MaterialRepository{db: db}
}

func (r *MaterialRepository) Create(ctx context.Context, material models.Material) (models.Material, error) {
	created, err := scanMaterial(r.db.QueryRow(ctx, `
		INSERT INTO materiales (nombre, tipo, marca, color, costo_por_kg, stock_kg, activo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, nombre, tipo, marca, color, costo_por_kg, stock_kg, activo, fecha_actualizacion`,
		material.Name, material.Type, material.Brand, material.Color, material.CostPerKG, material.StockKG, material.Active))
	return created, translateError(err)
}

func (r *MaterialRepository) List(ctx context.Context, filters MaterialFilters) ([]models.Material, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, nombre, tipo, marca, color, costo_por_kg, stock_kg, activo, fecha_actualizacion
		FROM materiales
		WHERE ($1::text IS NULL OR tipo ILIKE $1)
			AND ($2::text IS NULL OR marca ILIKE $2)
			AND ($3::text IS NULL OR color ILIKE $3)
			AND ($4::boolean IS NULL OR activo = $4)
		ORDER BY nombre, id`, filters.Type, filters.Brand, filters.Color, filters.Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	materials := make([]models.Material, 0)
	for rows.Next() {
		material, err := scanMaterial(rows)
		if err != nil {
			return nil, err
		}
		materials = append(materials, material)
	}
	return materials, rows.Err()
}

func (r *MaterialRepository) Get(ctx context.Context, id int64) (models.Material, error) {
	material, err := scanMaterial(r.db.QueryRow(ctx, `
		SELECT id, nombre, tipo, marca, color, costo_por_kg, stock_kg, activo, fecha_actualizacion
		FROM materiales
		WHERE id = $1`, id))
	return material, translateError(err)
}

func (r *MaterialRepository) Update(ctx context.Context, material models.Material) (models.Material, error) {
	updated, err := scanMaterial(r.db.QueryRow(ctx, `
		UPDATE materiales
		SET nombre = $2, tipo = $3, marca = $4, color = $5, costo_por_kg = $6,
			stock_kg = $7, activo = $8, fecha_actualizacion = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING id, nombre, tipo, marca, color, costo_por_kg, stock_kg, activo, fecha_actualizacion`,
		material.ID, material.Name, material.Type, material.Brand, material.Color,
		material.CostPerKG, material.StockKG, material.Active))
	return updated, translateError(err)
}

type RateRepository struct {
	db DB
}

func NewRateRepository(db DB) *RateRepository {
	return &RateRepository{db: db}
}

func (r *RateRepository) MachineExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM maquinas WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (r *RateRepository) LocationExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM locaciones WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

func (r *RateRepository) GetMachineRate(ctx context.Context, machineID int64) (models.MachineRate, error) {
	rate, err := scanMachineRate(r.db.QueryRow(ctx, `
		SELECT id, maquina_id, costo_interno_hora, precio_venta_hora, costo_preparacion, fecha_actualizacion
		FROM tarifas_maquina
		WHERE maquina_id = $1`, machineID))
	return rate, translateError(err)
}

func (r *RateRepository) UpsertMachineRate(ctx context.Context, rate models.MachineRate) (models.MachineRate, error) {
	updated, err := scanMachineRate(r.db.QueryRow(ctx, `
		INSERT INTO tarifas_maquina (
			maquina_id, costo_interno_hora, precio_venta_hora, costo_preparacion
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (maquina_id) DO UPDATE SET
			costo_interno_hora = EXCLUDED.costo_interno_hora,
			precio_venta_hora = EXCLUDED.precio_venta_hora,
			costo_preparacion = EXCLUDED.costo_preparacion,
			fecha_actualizacion = CURRENT_TIMESTAMP
		RETURNING id, maquina_id, costo_interno_hora, precio_venta_hora, costo_preparacion, fecha_actualizacion`,
		rate.MachineID, rate.InternalCostHour, rate.SalePriceHour, rate.PreparationCost))
	return updated, translateError(err)
}

func (r *RateRepository) GetEnergyRate(ctx context.Context, locationID int64) (models.EnergyRate, error) {
	rate, err := scanEnergyRate(r.db.QueryRow(ctx, `
		SELECT id, locacion_id, costo_por_kwh, fecha_actualizacion
		FROM tarifas_energia
		WHERE locacion_id = $1`, locationID))
	return rate, translateError(err)
}

func (r *RateRepository) UpsertEnergyRate(ctx context.Context, rate models.EnergyRate) (models.EnergyRate, error) {
	updated, err := scanEnergyRate(r.db.QueryRow(ctx, `
		INSERT INTO tarifas_energia (locacion_id, costo_por_kwh)
		VALUES ($1, $2)
		ON CONFLICT (locacion_id) DO UPDATE SET
			costo_por_kwh = EXCLUDED.costo_por_kwh,
			fecha_actualizacion = CURRENT_TIMESTAMP
		RETURNING id, locacion_id, costo_por_kwh, fecha_actualizacion`, rate.LocationID, rate.CostPerKWh))
	return updated, translateError(err)
}

func scanMaterial(row scanner) (models.Material, error) {
	var material models.Material
	err := row.Scan(
		&material.ID,
		&material.Name,
		&material.Type,
		&material.Brand,
		&material.Color,
		&material.CostPerKG,
		&material.StockKG,
		&material.Active,
		&material.LastUpdatedAt,
	)
	return material, err
}

func scanMachineRate(row scanner) (models.MachineRate, error) {
	var rate models.MachineRate
	err := row.Scan(
		&rate.ID,
		&rate.MachineID,
		&rate.InternalCostHour,
		&rate.SalePriceHour,
		&rate.PreparationCost,
		&rate.LastUpdatedAt,
	)
	return rate, err
}

func scanEnergyRate(row scanner) (models.EnergyRate, error) {
	var rate models.EnergyRate
	err := row.Scan(&rate.ID, &rate.LocationID, &rate.CostPerKWh, &rate.LastUpdatedAt)
	return rate, err
}
