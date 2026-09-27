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

type QuoteData struct {
	MachineID         int64
	MachineCode       string
	MachineName       string
	MachineActive     bool
	LocationID        int64
	PowerWatts        *string
	MaterialID        int64
	MaterialName      string
	MaterialActive    bool
	MaterialCostPerKG string
	InternalCostHour  *string
	SalePriceHour     *string
	PreparationCost   *string
	EnergyCostPerKWh  *string
}

type QuoteConceptSnapshot struct {
	Description *string
	Calculation models.QuoteCalculation
	Rates       QuoteData
}

type CreateQuoteData struct {
	CustomerID          int64
	ExpirationDate      *time.Time
	Notes               *string
	TotalCost           models.Money
	TotalSuggestedPrice models.Money
	Concepts            []QuoteConceptSnapshot
}

type QuoteFilters struct {
	CustomerID *int64
	StatusCode *string
}

type quoteTransaction interface {
	QueryRow(context.Context, string, ...any) scanner
	Commit(context.Context) error
	Rollback(context.Context) error
}

type quoteTransactionManager interface {
	Begin(context.Context) (quoteTransaction, error)
}

type pgxQuoteTransactionManager struct {
	pool *pgxpool.Pool
}

func (m pgxQuoteTransactionManager) Begin(ctx context.Context) (quoteTransaction, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxQuoteTransaction{Tx: tx}, nil
}

type pgxQuoteTransaction struct {
	pgx.Tx
}

func (tx pgxQuoteTransaction) QueryRow(ctx context.Context, query string, args ...any) scanner {
	return tx.Tx.QueryRow(ctx, query, args...)
}

type QuoteRepository struct {
	db           DB
	transactions quoteTransactionManager
}

func NewQuoteRepository(pool *pgxpool.Pool) *QuoteRepository {
	return &QuoteRepository{db: pool, transactions: pgxQuoteTransactionManager{pool: pool}}
}

func (r *QuoteRepository) GetCalculationData(ctx context.Context, machineID, materialID int64) (QuoteData, error) {
	var data QuoteData
	err := r.db.QueryRow(ctx, `
		SELECT m.id, m.codigo, m.nombre, m.activa, m.locacion_id, m.potencia_watts::text,
			mat.id, mat.nombre, mat.activo, mat.costo_por_kg::text,
			tm.costo_interno_hora::text, tm.precio_venta_hora::text,
			tm.costo_preparacion::text, te.costo_por_kwh::text
		FROM maquinas m
		JOIN materiales mat ON mat.id = $2
		LEFT JOIN tarifas_maquina tm ON tm.maquina_id = m.id
		LEFT JOIN tarifas_energia te ON te.locacion_id = m.locacion_id
		WHERE m.id = $1`, machineID, materialID).Scan(
		&data.MachineID,
		&data.MachineCode,
		&data.MachineName,
		&data.MachineActive,
		&data.LocationID,
		&data.PowerWatts,
		&data.MaterialID,
		&data.MaterialName,
		&data.MaterialActive,
		&data.MaterialCostPerKG,
		&data.InternalCostHour,
		&data.SalePriceHour,
		&data.PreparationCost,
		&data.EnergyCostPerKWh,
	)
	return data, translateError(err)
}

func (r *QuoteRepository) ActiveCustomerExists(ctx context.Context, customerID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clientes WHERE id = $1 AND activo)`, customerID).Scan(&exists)
	return exists, err
}

func (r *QuoteRepository) Create(ctx context.Context, data CreateQuoteData) (models.Quote, error) {
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return models.Quote{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	quote, err := scanQuote(tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO cotizaciones (
				cliente_id, estado_cotizacion_id, fecha_vencimiento, notas,
				costo_total, precio_sugerido_total
			)
			SELECT $1, e.id, $2, $3, $4::numeric / 100, $5::numeric / 100
			FROM estados_cotizacion e
			WHERE e.codigo = 'BORRADOR'
			RETURNING *
		)
		SELECT i.id, i.cliente_id, c.nombre, i.estado_cotizacion_id, e.codigo, e.nombre,
			i.fecha_vencimiento, i.notas, ROUND(i.costo_total * 100)::bigint,
			ROUND(i.precio_sugerido_total * 100)::bigint, i.fecha_creacion, i.fecha_actualizacion
		FROM inserted i
		JOIN clientes c ON c.id = i.cliente_id
		JOIN estados_cotizacion e ON e.id = i.estado_cotizacion_id`, data.CustomerID,
		data.ExpirationDate, data.Notes, int64(data.TotalCost), int64(data.TotalSuggestedPrice)))
	if err != nil {
		return models.Quote{}, translateError(err)
	}
	quote.Concepts = make([]models.QuoteConcept, 0, len(data.Concepts))

	for _, concept := range data.Concepts {
		calculation := concept.Calculation
		rates := concept.Rates
		var conceptID int64
		var creationDate time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO conceptos_cotizacion (
				cotizacion_id, maquina_id, material_id, maquina_codigo, maquina_nombre,
				material_nombre, descripcion, cantidad_material_gramos, duracion_minutos,
				cantidad_piezas, costo_material_por_kg, costo_interno_hora,
				precio_venta_hora, tarifa_costo_preparacion, potencia_watts,
				costo_por_kwh, costo_material, costo_maquina, costo_electrico,
				costo_preparacion, subtotal, precio_sugerido, precio_sugerido_por_pieza
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
				$15, $16, $17::numeric / 100, $18::numeric / 100, $19::numeric / 100,
				$20::numeric / 100, $21::numeric / 100, $22::numeric / 100, $23::numeric / 100
			)
			RETURNING id, fecha_creacion`, quote.ID, calculation.MachineID, calculation.MaterialID, rates.MachineCode,
			rates.MachineName, rates.MaterialName, concept.Description, calculation.MaterialGrams.String(),
			calculation.DurationMinutes, calculation.PieceCount, rates.MaterialCostPerKG,
			*rates.InternalCostHour, *rates.SalePriceHour, *rates.PreparationCost,
			*rates.PowerWatts, *rates.EnergyCostPerKWh, int64(calculation.MaterialCost),
			int64(calculation.MachineCost), int64(calculation.ElectricityCost),
			int64(calculation.PreparationCost), int64(calculation.Subtotal),
			int64(calculation.SuggestedPrice), int64(calculation.SuggestedPricePerPiece)).Scan(&conceptID, &creationDate)
		if err != nil {
			return models.Quote{}, translateError(err)
		}
		quote.Concepts = append(quote.Concepts, models.QuoteConcept{
			ID: conceptID, QuoteID: quote.ID, MachineID: calculation.MachineID,
			MaterialID: calculation.MaterialID, MachineCode: rates.MachineCode,
			MachineName: rates.MachineName, MaterialName: rates.MaterialName,
			Description: concept.Description, MaterialGrams: calculation.MaterialGrams,
			DurationMinutes: calculation.DurationMinutes, PieceCount: calculation.PieceCount,
			MaterialCostPerKG: json.Number(rates.MaterialCostPerKG),
			InternalCostHour:  json.Number(*rates.InternalCostHour), SalePriceHour: json.Number(*rates.SalePriceHour),
			RatePreparationCost: json.Number(*rates.PreparationCost), PowerWatts: json.Number(*rates.PowerWatts),
			EnergyCostPerKWh: json.Number(*rates.EnergyCostPerKWh), MaterialCost: calculation.MaterialCost,
			MachineCost: calculation.MachineCost, ElectricityCost: calculation.ElectricityCost,
			PreparationCost: calculation.PreparationCost, Subtotal: calculation.Subtotal,
			SuggestedPrice: calculation.SuggestedPrice, SuggestedPricePerPiece: calculation.SuggestedPricePerPiece,
			CreationDate: creationDate,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Quote{}, err
	}
	return quote, nil
}

func (r *QuoteRepository) List(ctx context.Context, filters QuoteFilters) ([]models.Quote, error) {
	rows, err := r.db.Query(ctx, `
		SELECT q.id, q.cliente_id, c.nombre, q.estado_cotizacion_id, e.codigo, e.nombre,
			q.fecha_vencimiento, q.notas, ROUND(q.costo_total * 100)::bigint,
			ROUND(q.precio_sugerido_total * 100)::bigint, q.fecha_creacion, q.fecha_actualizacion
		FROM cotizaciones q
		JOIN clientes c ON c.id = q.cliente_id
		JOIN estados_cotizacion e ON e.id = q.estado_cotizacion_id
		WHERE ($1::bigint IS NULL OR q.cliente_id = $1)
			AND ($2::text IS NULL OR e.codigo = $2)
		ORDER BY q.fecha_creacion DESC, q.id DESC`, filters.CustomerID, filters.StatusCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quotes := make([]models.Quote, 0)
	for rows.Next() {
		quote, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		quotes = append(quotes, quote)
	}
	return quotes, rows.Err()
}

func (r *QuoteRepository) Get(ctx context.Context, id int64) (models.Quote, error) {
	quote, err := scanQuote(r.db.QueryRow(ctx, `
		SELECT q.id, q.cliente_id, c.nombre, q.estado_cotizacion_id, e.codigo, e.nombre,
			q.fecha_vencimiento, q.notas, ROUND(q.costo_total * 100)::bigint,
			ROUND(q.precio_sugerido_total * 100)::bigint, q.fecha_creacion, q.fecha_actualizacion
		FROM cotizaciones q
		JOIN clientes c ON c.id = q.cliente_id
		JOIN estados_cotizacion e ON e.id = q.estado_cotizacion_id
		WHERE q.id = $1`, id))
	if err != nil {
		return models.Quote{}, translateError(err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, cotizacion_id, maquina_id, material_id, maquina_codigo, maquina_nombre,
			material_nombre, descripcion, cantidad_material_gramos::text, duracion_minutos,
			cantidad_piezas, costo_material_por_kg::text, costo_interno_hora::text,
			precio_venta_hora::text, tarifa_costo_preparacion::text, potencia_watts::text,
			costo_por_kwh::text, ROUND(costo_material * 100)::bigint,
			ROUND(costo_maquina * 100)::bigint, ROUND(costo_electrico * 100)::bigint,
			ROUND(costo_preparacion * 100)::bigint, ROUND(subtotal * 100)::bigint,
			ROUND(precio_sugerido * 100)::bigint,
			ROUND(precio_sugerido_por_pieza * 100)::bigint, fecha_creacion
		FROM conceptos_cotizacion
		WHERE cotizacion_id = $1
		ORDER BY id`, id)
	if err != nil {
		return models.Quote{}, err
	}
	defer rows.Close()
	quote.Concepts = make([]models.QuoteConcept, 0)
	for rows.Next() {
		concept, err := scanQuoteConcept(rows)
		if err != nil {
			return models.Quote{}, err
		}
		quote.Concepts = append(quote.Concepts, concept)
	}
	return quote, rows.Err()
}

func (r *QuoteRepository) ListStatuses(ctx context.Context) ([]models.QuoteStatus, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, codigo, nombre, descripcion
		FROM estados_cotizacion
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	statuses := make([]models.QuoteStatus, 0)
	for rows.Next() {
		var status models.QuoteStatus
		if err := rows.Scan(&status.ID, &status.Code, &status.Name, &status.Description); err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (r *QuoteRepository) ChangeStatus(ctx context.Context, id, currentStatusID int64, statusCode string) (models.Quote, error) {
	quote, err := scanQuote(r.db.QueryRow(ctx, `
		WITH updated AS (
			UPDATE cotizaciones q
			SET estado_cotizacion_id = target.id, fecha_actualizacion = CURRENT_TIMESTAMP
			FROM estados_cotizacion target
			WHERE q.id = $1 AND q.estado_cotizacion_id = $2 AND target.codigo = $3
			RETURNING q.*
		)
		SELECT u.id, u.cliente_id, c.nombre, u.estado_cotizacion_id, e.codigo, e.nombre,
			u.fecha_vencimiento, u.notas, ROUND(u.costo_total * 100)::bigint,
			ROUND(u.precio_sugerido_total * 100)::bigint, u.fecha_creacion, u.fecha_actualizacion
		FROM updated u
		JOIN clientes c ON c.id = u.cliente_id
		JOIN estados_cotizacion e ON e.id = u.estado_cotizacion_id`, id, currentStatusID, statusCode))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Quote{}, ErrConflict
	}
	return quote, translateError(err)
}

func scanQuote(row scanner) (models.Quote, error) {
	var quote models.Quote
	var totalCost, totalSuggestedPrice int64
	err := row.Scan(
		&quote.ID,
		&quote.CustomerID,
		&quote.CustomerName,
		&quote.StatusID,
		&quote.StatusCode,
		&quote.StatusName,
		&quote.ExpirationDate,
		&quote.Notes,
		&totalCost,
		&totalSuggestedPrice,
		&quote.CreationDate,
		&quote.LastUpdatedAt,
	)
	quote.TotalCost = models.Money(totalCost)
	quote.TotalSuggestedPrice = models.Money(totalSuggestedPrice)
	return quote, err
}

func scanQuoteConcept(row scanner) (models.QuoteConcept, error) {
	var concept models.QuoteConcept
	var grams, materialCostPerKG, internalCostHour, salePriceHour string
	var preparationCost, powerWatts, energyCostPerKWh string
	var materialCost, machineCost, electricityCost, chargedPreparationCost int64
	var subtotal, suggestedPrice, suggestedPricePerPiece int64
	err := row.Scan(
		&concept.ID,
		&concept.QuoteID,
		&concept.MachineID,
		&concept.MaterialID,
		&concept.MachineCode,
		&concept.MachineName,
		&concept.MaterialName,
		&concept.Description,
		&grams,
		&concept.DurationMinutes,
		&concept.PieceCount,
		&materialCostPerKG,
		&internalCostHour,
		&salePriceHour,
		&preparationCost,
		&powerWatts,
		&energyCostPerKWh,
		&materialCost,
		&machineCost,
		&electricityCost,
		&chargedPreparationCost,
		&subtotal,
		&suggestedPrice,
		&suggestedPricePerPiece,
		&concept.CreationDate,
	)
	concept.MaterialGrams = json.Number(grams)
	concept.MaterialCostPerKG = json.Number(materialCostPerKG)
	concept.InternalCostHour = json.Number(internalCostHour)
	concept.SalePriceHour = json.Number(salePriceHour)
	concept.RatePreparationCost = json.Number(preparationCost)
	concept.PowerWatts = json.Number(powerWatts)
	concept.EnergyCostPerKWh = json.Number(energyCostPerKWh)
	concept.MaterialCost = models.Money(materialCost)
	concept.MachineCost = models.Money(machineCost)
	concept.ElectricityCost = models.Money(electricityCost)
	concept.PreparationCost = models.Money(chargedPreparationCost)
	concept.Subtotal = models.Money(subtotal)
	concept.SuggestedPrice = models.Money(suggestedPrice)
	concept.SuggestedPricePerPiece = models.Money(suggestedPricePerPiece)
	return concept, err
}
