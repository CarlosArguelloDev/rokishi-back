package repository

import (
	"context"

	"rokishi-back/internal/models"
)

type CustomerFilters struct {
	Active *bool
	Query  *string
}

type CustomerRepository struct {
	db DB
}

func NewCustomerRepository(db DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, customer models.Customer) (models.Customer, error) {
	created, err := scanCustomer(r.db.QueryRow(ctx, `
		INSERT INTO clientes (nombre, correo, telefono, notas, activo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, nombre, correo, telefono, notas, activo, fecha_creacion, fecha_actualizacion`,
		customer.Name, customer.Email, customer.Phone, customer.Notes, customer.Active))
	return created, translateError(err)
}

func (r *CustomerRepository) List(ctx context.Context, filters CustomerFilters) ([]models.Customer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, nombre, correo, telefono, notas, activo, fecha_creacion, fecha_actualizacion
		FROM clientes
		WHERE ($1::boolean IS NULL OR activo = $1)
			AND ($2::text IS NULL OR nombre ILIKE '%' || $2 || '%'
				OR correo ILIKE '%' || $2 || '%' OR telefono ILIKE '%' || $2 || '%')
		ORDER BY nombre, id`, filters.Active, filters.Query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customers := make([]models.Customer, 0)
	for rows.Next() {
		customer, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func (r *CustomerRepository) Get(ctx context.Context, id int64) (models.Customer, error) {
	customer, err := scanCustomer(r.db.QueryRow(ctx, `
		SELECT id, nombre, correo, telefono, notas, activo, fecha_creacion, fecha_actualizacion
		FROM clientes
		WHERE id = $1`, id))
	return customer, translateError(err)
}

func (r *CustomerRepository) Update(ctx context.Context, customer models.Customer) (models.Customer, error) {
	updated, err := scanCustomer(r.db.QueryRow(ctx, `
		UPDATE clientes
		SET nombre = $2, correo = $3, telefono = $4, notas = $5, activo = $6,
			fecha_actualizacion = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING id, nombre, correo, telefono, notas, activo, fecha_creacion, fecha_actualizacion`,
		customer.ID, customer.Name, customer.Email, customer.Phone, customer.Notes, customer.Active))
	return updated, translateError(err)
}

func scanCustomer(row scanner) (models.Customer, error) {
	var customer models.Customer
	err := row.Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Notes,
		&customer.Active,
		&customer.CreationDate,
		&customer.LastUpdatedAt,
	)
	return customer, err
}
