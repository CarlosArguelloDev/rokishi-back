package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"rokishi-back/internal/models"
)

type StateHistoryFilters struct {
	From *time.Time
	To   *time.Time
}

type stateTransaction interface {
	QueryRow(context.Context, string, ...any) scanner
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type stateTransactionManager interface {
	Begin(context.Context) (stateTransaction, error)
}

type pgxStateTransactionManager struct {
	pool *pgxpool.Pool
}

func (m pgxStateTransactionManager) Begin(ctx context.Context) (stateTransaction, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxStateTransaction{Tx: tx}, nil
}

type pgxStateTransaction struct {
	pgx.Tx
}

func (tx pgxStateTransaction) QueryRow(ctx context.Context, query string, args ...any) scanner {
	return tx.Tx.QueryRow(ctx, query, args...)
}

type MachineStateRepository struct {
	db           DB
	transactions stateTransactionManager
}

func NewMachineStateRepository(pool *pgxpool.Pool) *MachineStateRepository {
	return &MachineStateRepository{db: pool, transactions: pgxStateTransactionManager{pool: pool}}
}

func (r *MachineStateRepository) ListStates(ctx context.Context) ([]models.MachineState, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, codigo, nombre, descripcion
		FROM estados_maquina
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]models.MachineState, 0)
	for rows.Next() {
		state, err := scanMachineState(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (r *MachineStateRepository) MachineExists(ctx context.Context, machineID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM maquinas WHERE id = $1)`, machineID).Scan(&exists)
	return exists, err
}

func (r *MachineStateRepository) Current(ctx context.Context, machineID int64) (models.MachineStatePeriod, error) {
	period, err := scanMachineStatePeriod(r.db.QueryRow(ctx, `
		SELECT h.id, h.maquina_id, h.trabajo_id, h.estado_maquina_id, e.codigo, e.nombre,
			h.fecha_inicio, h.fecha_fin, h.notas
		FROM historial_estados_maquina h
		JOIN estados_maquina e ON e.id = h.estado_maquina_id
		WHERE h.maquina_id = $1 AND h.fecha_fin IS NULL`, machineID))
	return period, translateError(err)
}

func (r *MachineStateRepository) History(ctx context.Context, machineID int64, filters StateHistoryFilters) ([]models.MachineStatePeriod, error) {
	rows, err := r.db.Query(ctx, `
		SELECT h.id, h.maquina_id, h.trabajo_id, h.estado_maquina_id, e.codigo, e.nombre,
			h.fecha_inicio, h.fecha_fin, h.notas
		FROM historial_estados_maquina h
		JOIN estados_maquina e ON e.id = h.estado_maquina_id
		WHERE h.maquina_id = $1
			AND ($2::timestamptz IS NULL OR COALESCE(h.fecha_fin, 'infinity'::timestamptz) >= $2)
			AND ($3::timestamptz IS NULL OR h.fecha_inicio <= $3)
		ORDER BY h.fecha_inicio DESC, h.id DESC`, machineID, filters.From, filters.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]models.MachineStatePeriod, 0)
	for rows.Next() {
		period, err := scanMachineStatePeriod(rows)
		if err != nil {
			return nil, err
		}
		history = append(history, period)
	}
	return history, rows.Err()
}

func (r *MachineStateRepository) Change(ctx context.Context, machineID, stateID int64, startedAt time.Time, notes *string) (models.MachineStateChange, error) {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return models.MachineStateChange{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedMachineID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM maquinas WHERE id = $1 FOR UPDATE`, machineID).Scan(&lockedMachineID); err != nil {
		return models.MachineStateChange{}, translateError(err)
	}

	if _, err := scanMachineState(tx.QueryRow(ctx, `
		SELECT id, codigo, nombre, descripcion
		FROM estados_maquina
		WHERE id = $1`, stateID)); err != nil {
		return models.MachineStateChange{}, translateError(err)
	}

	previous, err := scanMachineStatePeriod(tx.QueryRow(ctx, `
		SELECT h.id, h.maquina_id, h.trabajo_id, h.estado_maquina_id, e.codigo, e.nombre,
			h.fecha_inicio, h.fecha_fin, h.notas
		FROM historial_estados_maquina h
		JOIN estados_maquina e ON e.id = h.estado_maquina_id
		WHERE h.maquina_id = $1 AND h.fecha_fin IS NULL
		FOR UPDATE OF h`, machineID))
	var previousPointer *models.MachineStatePeriod
	switch {
	case err == nil:
		previousPointer = &previous
		if previous.WorkID != nil {
			return models.MachineStateChange{}, ErrInvalidOperation
		}
		if previous.StateID == stateID {
			return models.MachineStateChange{}, ErrStateUnchanged
		}
		if !startedAt.After(previous.StartedAt) {
			return models.MachineStateChange{}, ErrInvalidStateTime
		}
		tag, updateErr := tx.Exec(ctx, `
			UPDATE historial_estados_maquina
			SET fecha_fin = $2
			WHERE id = $1 AND fecha_fin IS NULL`, previous.ID, startedAt)
		if updateErr != nil {
			return models.MachineStateChange{}, updateErr
		}
		if tag.RowsAffected() != 1 {
			return models.MachineStateChange{}, fmt.Errorf("no se pudo cerrar el estado actual")
		}
	case errors.Is(err, pgx.ErrNoRows):
		previousPointer = nil
	default:
		return models.MachineStateChange{}, err
	}

	current, err := scanMachineStatePeriod(tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO historial_estados_maquina (
				maquina_id, estado_maquina_id, fecha_inicio, notas
			)
			VALUES ($1, $2, $3, $4)
			RETURNING *
		)
		SELECT i.id, i.maquina_id, i.trabajo_id, i.estado_maquina_id, e.codigo, e.nombre,
			i.fecha_inicio, i.fecha_fin, i.notas
		FROM inserted i
		JOIN estados_maquina e ON e.id = i.estado_maquina_id`, machineID, stateID, startedAt, notes))
	if err != nil {
		return models.MachineStateChange{}, translateError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.MachineStateChange{}, err
	}
	return models.MachineStateChange{Previous: previousPointer, Current: current}, nil
}

func scanMachineState(row scanner) (models.MachineState, error) {
	var state models.MachineState
	err := row.Scan(&state.ID, &state.Code, &state.Name, &state.Description)
	return state, err
}

func scanMachineStatePeriod(row scanner) (models.MachineStatePeriod, error) {
	var period models.MachineStatePeriod
	err := row.Scan(
		&period.ID,
		&period.MachineID,
		&period.WorkID,
		&period.StateID,
		&period.StateCode,
		&period.StateName,
		&period.StartedAt,
		&period.EndedAt,
		&period.Notes,
	)
	return period, err
}
