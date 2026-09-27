package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"rokishi-back/internal/models"
)

type OrderFilters struct {
	CustomerID *int64
	Status     *string
}

type MachineAssignmentData struct {
	Active        bool
	MachineTypeID int64
}

type FinishWorkData struct {
	Result           string
	ConsumedMaterial string
	Waste            string
	Notes            *string
	FinishedAt       time.Time
}

type ProductionRepository struct {
	db           DB
	transactions stateTransactionManager
}

func NewProductionRepository(pool *pgxpool.Pool) *ProductionRepository {
	return &ProductionRepository{db: pool, transactions: pgxStateTransactionManager{pool: pool}}
}

func (r *ProductionRepository) CreateOrder(ctx context.Context, quoteID int64) (models.Order, error) {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return models.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedQuoteID int64
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT q.id, e.codigo
		FROM cotizaciones q
		JOIN estados_cotizacion e ON e.id = q.estado_cotizacion_id
		WHERE q.id = $1
		FOR UPDATE OF q`, quoteID).Scan(&lockedQuoteID, &status); err != nil {
		return models.Order{}, translateError(err)
	}
	if status != "ACEPTADA" {
		return models.Order{}, ErrInvalidOperation
	}

	var orderID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO pedidos (cotizacion_id)
		VALUES ($1)
		RETURNING id`, quoteID).Scan(&orderID); err != nil {
		return models.Order{}, translateError(err)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO trabajos (
			pedido_id, concepto_cotizacion_id, tipo_maquina_id_requerido,
			maquina_id, material_id, descripcion, cantidad_piezas,
			duracion_estimada_minutos, material_estimado_gramos
		)
		SELECT $1, cc.id, m.tipo_maquina_id, cc.maquina_id, cc.material_id,
			cc.descripcion, cc.cantidad_piezas, cc.duracion_minutos,
			cc.cantidad_material_gramos
		FROM conceptos_cotizacion cc
		JOIN maquinas m ON m.id = cc.maquina_id
		WHERE cc.cotizacion_id = $2`, orderID, quoteID)
	if err != nil {
		return models.Order{}, translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return models.Order{}, ErrReferenceMissing
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Order{}, err
	}
	return r.GetOrder(ctx, orderID)
}

func (r *ProductionRepository) ListOrders(ctx context.Context, filters OrderFilters) ([]models.Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.cotizacion_id, q.cliente_id, c.nombre, p.estado,
			COUNT(t.id), COUNT(t.id) FILTER (WHERE t.estado = 'COMPLETADO'),
			p.fecha_creacion, p.fecha_actualizacion
		FROM pedidos p
		JOIN cotizaciones q ON q.id = p.cotizacion_id
		JOIN clientes c ON c.id = q.cliente_id
		LEFT JOIN trabajos t ON t.pedido_id = p.id
		WHERE ($1::bigint IS NULL OR q.cliente_id = $1)
			AND ($2::text IS NULL OR p.estado = $2)
		GROUP BY p.id, q.cliente_id, c.nombre
		ORDER BY p.fecha_creacion DESC, p.id DESC`, filters.CustomerID, filters.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]models.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *ProductionRepository) GetOrder(ctx context.Context, id int64) (models.Order, error) {
	order, err := scanOrder(r.db.QueryRow(ctx, `
		SELECT p.id, p.cotizacion_id, q.cliente_id, c.nombre, p.estado,
			COUNT(t.id), COUNT(t.id) FILTER (WHERE t.estado = 'COMPLETADO'),
			p.fecha_creacion, p.fecha_actualizacion
		FROM pedidos p
		JOIN cotizaciones q ON q.id = p.cotizacion_id
		JOIN clientes c ON c.id = q.cliente_id
		LEFT JOIN trabajos t ON t.pedido_id = p.id
		WHERE p.id = $1
		GROUP BY p.id, q.cliente_id, c.nombre`, id))
	if err != nil {
		return models.Order{}, translateError(err)
	}
	rows, err := r.db.Query(ctx, workSelect+` WHERE t.pedido_id = $1 ORDER BY t.id`, id)
	if err != nil {
		return models.Order{}, err
	}
	order.Works = make([]models.Work, 0)
	for rows.Next() {
		work, err := scanWork(rows)
		if err != nil {
			rows.Close()
			return models.Order{}, err
		}
		order.Works = append(order.Works, work)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return models.Order{}, err
	}
	rows.Close()
	for index := range order.Works {
		attempts, err := r.listAttempts(ctx, order.Works[index].ID)
		if err != nil {
			return models.Order{}, err
		}
		order.Works[index].Attempts = attempts
	}
	return order, nil
}

func (r *ProductionRepository) GetWork(ctx context.Context, id int64) (models.Work, error) {
	work, err := scanWork(r.db.QueryRow(ctx, workSelect+` WHERE t.id = $1`, id))
	if err != nil {
		return models.Work{}, translateError(err)
	}
	work.Attempts, err = r.listAttempts(ctx, id)
	return work, err
}

func (r *ProductionRepository) GetMachineAssignmentData(ctx context.Context, machineID int64) (MachineAssignmentData, error) {
	var data MachineAssignmentData
	err := r.db.QueryRow(ctx, `SELECT activa, tipo_maquina_id FROM maquinas WHERE id = $1`, machineID).Scan(&data.Active, &data.MachineTypeID)
	return data, translateError(err)
}

func (r *ProductionRepository) AssignMachine(ctx context.Context, workID, machineID int64) (models.Work, error) {
	var updatedID int64
	err := r.db.QueryRow(ctx, `
		UPDATE trabajos
		SET maquina_id = $2, fecha_actualizacion = CURRENT_TIMESTAMP
		WHERE id = $1 AND estado = 'PENDIENTE'
		RETURNING id`, workID, machineID).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Work{}, ErrInvalidOperation
	}
	if err != nil {
		return models.Work{}, translateError(err)
	}
	return r.GetWork(ctx, updatedID)
}

func (r *ProductionRepository) StartWork(ctx context.Context, workID int64, startedAt time.Time) error {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID, requiredTypeID int64
	var machineID *int64
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT pedido_id, estado, maquina_id, tipo_maquina_id_requerido
		FROM trabajos WHERE id = $1 FOR UPDATE`, workID).Scan(&orderID, &status, &machineID, &requiredTypeID); err != nil {
		return translateError(err)
	}
	if status != "PENDIENTE" || machineID == nil {
		return ErrInvalidOperation
	}
	var machineActive bool
	var machineTypeID int64
	if err := tx.QueryRow(ctx, `SELECT activa, tipo_maquina_id FROM maquinas WHERE id = $1 FOR UPDATE`, *machineID).Scan(&machineActive, &machineTypeID); err != nil {
		return translateError(err)
	}
	if !machineActive || machineTypeID != requiredTypeID {
		return ErrMachineUnavailable
	}

	var currentID int64
	var currentStart time.Time
	var currentCode string
	currentErr := tx.QueryRow(ctx, `
		SELECT h.id, h.fecha_inicio, e.codigo
		FROM historial_estados_maquina h
		JOIN estados_maquina e ON e.id = h.estado_maquina_id
		WHERE h.maquina_id = $1 AND h.fecha_fin IS NULL
		FOR UPDATE OF h`, *machineID).Scan(&currentID, &currentStart, &currentCode)
	switch {
	case currentErr == nil:
		if currentCode != "DISPONIBLE" || !startedAt.After(currentStart) {
			return ErrMachineUnavailable
		}
		if _, err := tx.Exec(ctx, `UPDATE historial_estados_maquina SET fecha_fin = $2 WHERE id = $1`, currentID, startedAt); err != nil {
			return err
		}
	case errors.Is(currentErr, pgx.ErrNoRows):
	default:
		return currentErr
	}

	var attemptID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO intentos_trabajo (trabajo_id, numero_intento, maquina_id, fecha_inicio)
		SELECT $1, COALESCE(MAX(numero_intento), 0) + 1, $2, $3
		FROM intentos_trabajo WHERE trabajo_id = $1
		RETURNING id`, workID, *machineID, startedAt).Scan(&attemptID); err != nil {
		return translateError(err)
	}
	var historyID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO historial_estados_maquina (maquina_id, estado_maquina_id, trabajo_id, fecha_inicio, notas)
		SELECT $1, id, $2, $3, $4 FROM estados_maquina WHERE codigo = 'TRABAJANDO'
		RETURNING id`, *machineID, workID, startedAt, "Trabajo iniciado desde pedido").Scan(&historyID); err != nil {
		return translateError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE trabajos SET estado = 'EN_PROCESO', fecha_actualizacion = $2 WHERE id = $1`, workID, startedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE pedidos SET estado = 'EN_PRODUCCION', fecha_actualizacion = $2 WHERE id = $1`, orderID, startedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ProductionRepository) FinishWork(ctx context.Context, workID int64, data FinishWorkData) error {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID, attemptID, machineID int64
	var status string
	var startedAt time.Time
	if err := tx.QueryRow(ctx, `
		SELECT t.pedido_id, t.estado, i.id, i.maquina_id, i.fecha_inicio
		FROM trabajos t
		JOIN intentos_trabajo i ON i.trabajo_id = t.id AND i.fecha_fin IS NULL
		WHERE t.id = $1
		FOR UPDATE OF t, i`, workID).Scan(&orderID, &status, &attemptID, &machineID, &startedAt); err != nil {
		return translateError(err)
	}
	if status != "EN_PROCESO" || !data.FinishedAt.After(startedAt) {
		return ErrInvalidOperation
	}
	var historyID int64
	var historyWorkID *int64
	var historyCode string
	if err := tx.QueryRow(ctx, `
		SELECT h.id, h.trabajo_id, e.codigo
		FROM historial_estados_maquina h
		JOIN estados_maquina e ON e.id = h.estado_maquina_id
		WHERE h.maquina_id = $1 AND h.fecha_fin IS NULL
		FOR UPDATE OF h`, machineID).Scan(&historyID, &historyWorkID, &historyCode); err != nil {
		return translateError(err)
	}
	if historyWorkID == nil || *historyWorkID != workID || historyCode != "TRABAJANDO" {
		return ErrInvalidOperation
	}
	if _, err := tx.Exec(ctx, `
		UPDATE intentos_trabajo
		SET fecha_fin = $2, resultado = $3, material_consumido_gramos = $4,
			desperdicio_gramos = $5, notas = $6
		WHERE id = $1 AND fecha_fin IS NULL`, attemptID, data.FinishedAt, data.Result,
		data.ConsumedMaterial, data.Waste, data.Notes); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE historial_estados_maquina SET fecha_fin = $2 WHERE id = $1`, historyID, data.FinishedAt); err != nil {
		return err
	}
	var availableHistoryID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO historial_estados_maquina (maquina_id, estado_maquina_id, fecha_inicio, notas)
		SELECT $1, id, $2, $3 FROM estados_maquina WHERE codigo = 'DISPONIBLE'
		RETURNING id`, machineID, data.FinishedAt, "Trabajo finalizado desde pedido").Scan(&availableHistoryID); err != nil {
		return translateError(err)
	}
	workStatus := "PENDIENTE"
	if data.Result == "EXITOSO" {
		workStatus = "COMPLETADO"
	}
	if _, err := tx.Exec(ctx, `UPDATE trabajos SET estado = $2, fecha_actualizacion = $3 WHERE id = $1`, workID, workStatus, data.FinishedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE pedidos p
		SET estado = CASE
			WHEN NOT EXISTS (SELECT 1 FROM trabajos t WHERE t.pedido_id = p.id AND t.estado <> 'COMPLETADO')
			THEN 'COMPLETADO' ELSE 'EN_PRODUCCION' END,
			fecha_actualizacion = $2
		WHERE p.id = $1`, orderID, data.FinishedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const workSelect = `
	SELECT t.id, t.pedido_id, t.concepto_cotizacion_id,
		t.tipo_maquina_id_requerido, tm.nombre, t.maquina_id, m.codigo, m.nombre,
		t.material_id, mat.nombre, t.descripcion, t.cantidad_piezas,
		t.duracion_estimada_minutos, t.material_estimado_gramos::text,
		t.estado, t.fecha_creacion, t.fecha_actualizacion
	FROM trabajos t
	JOIN tipos_maquina tm ON tm.id = t.tipo_maquina_id_requerido
	LEFT JOIN maquinas m ON m.id = t.maquina_id
	JOIN materiales mat ON mat.id = t.material_id`

func (r *ProductionRepository) listAttempts(ctx context.Context, workID int64) ([]models.WorkAttempt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.trabajo_id, i.numero_intento, i.maquina_id, m.codigo, m.nombre,
			i.fecha_inicio, i.fecha_fin,
			CASE WHEN i.fecha_fin IS NULL THEN NULL ELSE EXTRACT(EPOCH FROM (i.fecha_fin - i.fecha_inicio))::bigint END,
			i.resultado, i.material_consumido_gramos::text, i.desperdicio_gramos::text, i.notas
		FROM intentos_trabajo i
		JOIN maquinas m ON m.id = i.maquina_id
		WHERE i.trabajo_id = $1
		ORDER BY i.numero_intento`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := make([]models.WorkAttempt, 0)
	for rows.Next() {
		attempt, err := scanWorkAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func scanOrder(row scanner) (models.Order, error) {
	var order models.Order
	err := row.Scan(&order.ID, &order.QuoteID, &order.CustomerID, &order.CustomerName,
		&order.Status, &order.WorkCount, &order.CompletedCount, &order.CreationDate, &order.LastUpdatedAt)
	return order, err
}

func scanWork(row scanner) (models.Work, error) {
	var work models.Work
	var estimatedMaterial string
	err := row.Scan(&work.ID, &work.OrderID, &work.QuoteConceptID,
		&work.RequiredMachineTypeID, &work.RequiredMachineType, &work.MachineID,
		&work.MachineCode, &work.MachineName, &work.MaterialID, &work.MaterialName,
		&work.Description, &work.PieceCount, &work.EstimatedMinutes, &estimatedMaterial,
		&work.Status, &work.CreationDate, &work.LastUpdatedAt)
	work.EstimatedMaterial = json.Number(estimatedMaterial)
	return work, err
}

func scanWorkAttempt(row scanner) (models.WorkAttempt, error) {
	var attempt models.WorkAttempt
	var consumed, waste *string
	err := row.Scan(&attempt.ID, &attempt.WorkID, &attempt.AttemptNumber, &attempt.MachineID,
		&attempt.MachineCode, &attempt.MachineName, &attempt.StartedAt, &attempt.EndedAt,
		&attempt.ActualSeconds, &attempt.Result, &consumed, &waste, &attempt.Notes)
	if consumed != nil {
		value := json.Number(*consumed)
		attempt.ConsumedMaterial = &value
	}
	if waste != nil {
		value := json.Number(*waste)
		attempt.Waste = &value
	}
	return attempt, err
}
