CREATE UNIQUE INDEX IF NOT EXISTS uq_historial_estado_abierto_maquina
    ON historial_estados_maquina (maquina_id)
    WHERE fecha_fin IS NULL;
