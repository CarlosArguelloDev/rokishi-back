package models

import "time"

type MetricsPeriod struct {
	From time.Time `json:"desde"`
	To   time.Time `json:"hasta"`
}

type MetricsValues struct {
	WorkingHours             float64 `json:"horas_trabajando"`
	AvailableHours           float64 `json:"horas_disponibles"`
	OffHours                 float64 `json:"horas_apagadas"`
	MaintenanceHours         float64 `json:"horas_mantenimiento"`
	FailureHours             float64 `json:"horas_falla"`
	PausedHours              float64 `json:"horas_pausadas"`
	ProductiveHours          float64 `json:"horas_productivas"`
	UnproductiveHours        float64 `json:"horas_improductivas"`
	RecordedHours            float64 `json:"horas_registradas"`
	UtilizationPercentage    float64 `json:"porcentaje_utilizacion"`
	CompletedJobs            int64   `json:"trabajos_completados"`
	FailedAttempts           int64   `json:"intentos_fallidos"`
	SuccessRate              float64 `json:"tasa_exito"`
	ConsumedMaterialGrams    float64 `json:"material_consumido_gramos"`
	WastedMaterialGrams      float64 `json:"material_desperdiciado_gramos"`
	EstimatedRevenue         Money   `json:"ingresos_estimados"`
	EstimatedProfit          Money   `json:"utilidad_estimada"`
	JobsWithoutFinancialData int64   `json:"trabajos_sin_datos_financieros"`
}

type MachineMetrics struct {
	MachineID     int64  `json:"maquina_id"`
	MachineCode   string `json:"maquina_codigo"`
	MachineName   string `json:"maquina_nombre"`
	LocationID    int64  `json:"locacion_id"`
	LocationName  string `json:"locacion_nombre"`
	MachineTypeID int64  `json:"tipo_maquina_id"`
	MachineType   string `json:"tipo_maquina_nombre"`
	MetricsValues
}

type MetricsReport struct {
	Period      MetricsPeriod    `json:"periodo"`
	Summary     MetricsValues    `json:"resumen"`
	Machines    []MachineMetrics `json:"maquinas"`
	Limitations []string         `json:"limitaciones"`
}
