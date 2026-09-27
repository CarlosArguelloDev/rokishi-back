package repository

import (
	"context"
	"time"
)

type MetricsFilters struct {
	From          time.Time
	To            time.Time
	MachineID     *int64
	LocationID    *int64
	MachineTypeID *int64
}

type MachineMetricsData struct {
	MachineID                int64
	MachineCode              string
	MachineName              string
	LocationID               int64
	LocationName             string
	MachineTypeID            int64
	MachineTypeName          string
	RecordedSeconds          float64
	WorkingSeconds           float64
	AvailableSeconds         float64
	OffSeconds               float64
	MaintenanceSeconds       float64
	FailureSeconds           float64
	PausedSeconds            float64
	CompletedJobs            int64
	FailedAttempts           int64
	ConsumedMaterialGrams    float64
	WastedMaterialGrams      float64
	EstimatedRevenueCents    int64
	EstimatedProfitCents     int64
	JobsWithoutFinancialData int64
}

type MetricsRepository struct {
	db DB
}

func NewMetricsRepository(db DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

func (r *MetricsRepository) Summary(ctx context.Context, filters MetricsFilters) ([]MachineMetricsData, error) {
	rows, err := r.db.Query(ctx, `
		WITH selected_machines AS (
			SELECT m.id, m.codigo, m.nombre, m.locacion_id, l.nombre AS locacion_nombre,
				m.tipo_maquina_id, tm.nombre AS tipo_maquina_nombre
			FROM maquinas m
			JOIN locaciones l ON l.id = m.locacion_id
			JOIN tipos_maquina tm ON tm.id = m.tipo_maquina_id
			WHERE ($3::bigint IS NULL OR m.id = $3)
				AND ($4::bigint IS NULL OR m.locacion_id = $4)
				AND ($5::bigint IS NULL OR m.tipo_maquina_id = $5)
		), state_metrics AS (
			SELECT h.maquina_id,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))), 0)::double precision AS segundos_registrados,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'TRABAJANDO'), 0)::double precision AS segundos_trabajando,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'DISPONIBLE'), 0)::double precision AS segundos_disponible,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'APAGADA'), 0)::double precision AS segundos_apagada,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'MANTENIMIENTO'), 0)::double precision AS segundos_mantenimiento,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'FALLA'), 0)::double precision AS segundos_falla,
				COALESCE(SUM(EXTRACT(EPOCH FROM (
					LEAST(COALESCE(h.fecha_fin, $2), $2) - GREATEST(h.fecha_inicio, $1)
				))) FILTER (WHERE e.codigo = 'PAUSADA'), 0)::double precision AS segundos_pausada
			FROM historial_estados_maquina h
			JOIN selected_machines sm ON sm.id = h.maquina_id
			JOIN estados_maquina e ON e.id = h.estado_maquina_id
			WHERE h.fecha_inicio < $2 AND COALESCE(h.fecha_fin, $2) > $1
			GROUP BY h.maquina_id
		), production_metrics AS (
			SELECT i.maquina_id,
				COUNT(*) FILTER (WHERE i.resultado = 'EXITOSO') AS trabajos_completados,
				COUNT(*) FILTER (WHERE i.resultado = 'FALLIDO') AS intentos_fallidos,
				COALESCE(SUM(i.material_consumido_gramos), 0)::double precision AS material_consumido,
				COALESCE(SUM(i.desperdicio_gramos), 0)::double precision AS material_desperdiciado,
				COALESCE(ROUND(SUM(cc.precio_sugerido) FILTER (
					WHERE i.resultado = 'EXITOSO' AND cc.id IS NOT NULL
				) * 100), 0)::bigint AS ingresos_centavos,
				COALESCE(ROUND(SUM(cc.precio_sugerido - cc.subtotal) FILTER (
					WHERE i.resultado = 'EXITOSO' AND cc.id IS NOT NULL
				) * 100), 0)::bigint AS utilidad_centavos,
				COUNT(*) FILTER (WHERE i.resultado = 'EXITOSO' AND cc.id IS NULL) AS sin_datos_financieros
			FROM intentos_trabajo i
			JOIN selected_machines sm ON sm.id = i.maquina_id
			JOIN trabajos t ON t.id = i.trabajo_id
			LEFT JOIN conceptos_cotizacion cc ON cc.id = t.concepto_cotizacion_id
			WHERE i.fecha_fin >= $1 AND i.fecha_fin < $2
			GROUP BY i.maquina_id
		)
		SELECT sm.id, sm.codigo, sm.nombre, sm.locacion_id, sm.locacion_nombre,
			sm.tipo_maquina_id, sm.tipo_maquina_nombre,
			COALESCE(s.segundos_registrados, 0), COALESCE(s.segundos_trabajando, 0),
			COALESCE(s.segundos_disponible, 0), COALESCE(s.segundos_apagada, 0),
			COALESCE(s.segundos_mantenimiento, 0), COALESCE(s.segundos_falla, 0),
			COALESCE(s.segundos_pausada, 0), COALESCE(p.trabajos_completados, 0),
			COALESCE(p.intentos_fallidos, 0), COALESCE(p.material_consumido, 0),
			COALESCE(p.material_desperdiciado, 0), COALESCE(p.ingresos_centavos, 0),
			COALESCE(p.utilidad_centavos, 0), COALESCE(p.sin_datos_financieros, 0)
		FROM selected_machines sm
		LEFT JOIN state_metrics s ON s.maquina_id = sm.id
		LEFT JOIN production_metrics p ON p.maquina_id = sm.id
		ORDER BY sm.nombre, sm.id`, filters.From, filters.To, filters.MachineID,
		filters.LocationID, filters.MachineTypeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics := make([]MachineMetricsData, 0)
	for rows.Next() {
		var item MachineMetricsData
		if err := rows.Scan(
			&item.MachineID, &item.MachineCode, &item.MachineName,
			&item.LocationID, &item.LocationName, &item.MachineTypeID,
			&item.MachineTypeName, &item.RecordedSeconds, &item.WorkingSeconds,
			&item.AvailableSeconds, &item.OffSeconds, &item.MaintenanceSeconds,
			&item.FailureSeconds, &item.PausedSeconds, &item.CompletedJobs,
			&item.FailedAttempts, &item.ConsumedMaterialGrams,
			&item.WastedMaterialGrams, &item.EstimatedRevenueCents,
			&item.EstimatedProfitCents, &item.JobsWithoutFinancialData,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, item)
	}
	return metrics, rows.Err()
}
