CREATE TABLE pedidos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cotizacion_id BIGINT NOT NULL UNIQUE REFERENCES cotizaciones(id),
    estado VARCHAR(30) NOT NULL DEFAULT 'PENDIENTE',
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (estado IN ('PENDIENTE', 'EN_PRODUCCION', 'COMPLETADO', 'CANCELADO'))
);

CREATE TABLE trabajos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pedido_id BIGINT NOT NULL REFERENCES pedidos(id),
    concepto_cotizacion_id BIGINT NOT NULL UNIQUE REFERENCES conceptos_cotizacion(id),
    tipo_maquina_id_requerido BIGINT NOT NULL REFERENCES tipos_maquina(id),
    maquina_id BIGINT REFERENCES maquinas(id),
    material_id BIGINT NOT NULL REFERENCES materiales(id),
    descripcion VARCHAR(200),
    cantidad_piezas INTEGER NOT NULL,
    duracion_estimada_minutos BIGINT NOT NULL,
    material_estimado_gramos NUMERIC(12, 3) NOT NULL,
    estado VARCHAR(30) NOT NULL DEFAULT 'PENDIENTE',
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (cantidad_piezas > 0),
    CHECK (duracion_estimada_minutos > 0),
    CHECK (material_estimado_gramos > 0),
    CHECK (estado IN ('PENDIENTE', 'EN_PROCESO', 'COMPLETADO', 'CANCELADO'))
);

CREATE TABLE intentos_trabajo (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trabajo_id BIGINT NOT NULL REFERENCES trabajos(id),
    numero_intento INTEGER NOT NULL,
    maquina_id BIGINT NOT NULL REFERENCES maquinas(id),
    fecha_inicio TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_fin TIMESTAMPTZ,
    resultado VARCHAR(20),
    material_consumido_gramos NUMERIC(12, 3),
    desperdicio_gramos NUMERIC(12, 3),
    notas TEXT,
    UNIQUE (trabajo_id, numero_intento),
    CHECK (numero_intento > 0),
    CHECK (fecha_fin IS NULL OR fecha_fin > fecha_inicio),
    CHECK (resultado IS NULL OR resultado IN ('EXITOSO', 'FALLIDO')),
    CHECK ((fecha_fin IS NULL AND resultado IS NULL) OR (fecha_fin IS NOT NULL AND resultado IS NOT NULL)),
    CHECK (material_consumido_gramos IS NULL OR material_consumido_gramos >= 0),
    CHECK (desperdicio_gramos IS NULL OR desperdicio_gramos >= 0)
);

ALTER TABLE historial_estados_maquina
    ADD COLUMN trabajo_id BIGINT REFERENCES trabajos(id);

CREATE INDEX idx_pedidos_estado_fecha ON pedidos (estado, fecha_creacion DESC);
CREATE INDEX idx_trabajos_pedido ON trabajos (pedido_id, id);
CREATE INDEX idx_trabajos_maquina_estado ON trabajos (maquina_id, estado);
CREATE INDEX idx_intentos_trabajo ON intentos_trabajo (trabajo_id, numero_intento);
CREATE UNIQUE INDEX uq_intento_abierto_trabajo ON intentos_trabajo (trabajo_id) WHERE fecha_fin IS NULL;
CREATE UNIQUE INDEX uq_intento_abierto_maquina ON intentos_trabajo (maquina_id) WHERE fecha_fin IS NULL;
