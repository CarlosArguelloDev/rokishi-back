package repository

import "context"

type QuoteData struct {
	MachineID         int64
	MachineActive     bool
	LocationID        int64
	PowerWatts        *string
	MaterialID        int64
	MaterialActive    bool
	MaterialCostPerKG string
	InternalCostHour  *string
	SalePriceHour     *string
	PreparationCost   *string
	EnergyCostPerKWh  *string
}

type QuoteRepository struct {
	db DB
}

func NewQuoteRepository(db DB) *QuoteRepository {
	return &QuoteRepository{db: db}
}

func (r *QuoteRepository) GetCalculationData(ctx context.Context, machineID, materialID int64) (QuoteData, error) {
	var data QuoteData
	err := r.db.QueryRow(ctx, `
		SELECT m.id, m.activa, m.locacion_id, m.potencia_watts::text,
			mat.id, mat.activo, mat.costo_por_kg::text,
			tm.costo_interno_hora::text, tm.precio_venta_hora::text,
			tm.costo_preparacion::text, te.costo_por_kwh::text
		FROM maquinas m
		JOIN materiales mat ON mat.id = $2
		LEFT JOIN tarifas_maquina tm ON tm.maquina_id = m.id
		LEFT JOIN tarifas_energia te ON te.locacion_id = m.locacion_id
		WHERE m.id = $1`, machineID, materialID).Scan(
		&data.MachineID,
		&data.MachineActive,
		&data.LocationID,
		&data.PowerWatts,
		&data.MaterialID,
		&data.MaterialActive,
		&data.MaterialCostPerKG,
		&data.InternalCostHour,
		&data.SalePriceHour,
		&data.PreparationCost,
		&data.EnergyCostPerKWh,
	)
	return data, translateError(err)
}
