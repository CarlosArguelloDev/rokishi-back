CREATE TABLE IF NOT EXISTS clientes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    correo VARCHAR(254),
    telefono VARCHAR(30),
    notas TEXT,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS estados_cotizacion (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(30) NOT NULL UNIQUE,
    nombre VARCHAR(60) NOT NULL,
    descripcion TEXT
);

INSERT INTO estados_cotizacion (codigo, nombre, descripcion)
VALUES
    ('BORRADOR', 'Borrador', 'La cotizacion sigue en preparacion'),
    ('ENVIADA', 'Enviada', 'La cotizacion fue enviada al cliente'),
    ('ACEPTADA', 'Aceptada', 'El cliente acepto la cotizacion'),
    ('RECHAZADA', 'Rechazada', 'El cliente rechazo la cotizacion'),
    ('VENCIDA', 'Vencida', 'La vigencia de la cotizacion termino'),
    ('CANCELADA', 'Cancelada', 'La cotizacion fue cancelada')
ON CONFLICT (codigo) DO NOTHING;

CREATE TABLE IF NOT EXISTS cotizaciones (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cliente_id BIGINT NOT NULL REFERENCES clientes(id),
    estado_cotizacion_id BIGINT NOT NULL REFERENCES estados_cotizacion(id),
    fecha_vencimiento TIMESTAMPTZ,
    notas TEXT,
    costo_total NUMERIC(14, 2) NOT NULL,
    precio_sugerido_total NUMERIC(14, 2) NOT NULL,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (costo_total >= 0),
    CHECK (precio_sugerido_total >= 0)
);

CREATE TABLE IF NOT EXISTS conceptos_cotizacion (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cotizacion_id BIGINT NOT NULL REFERENCES cotizaciones(id),
    maquina_id BIGINT NOT NULL REFERENCES maquinas(id),
    material_id BIGINT NOT NULL REFERENCES materiales(id),
    maquina_codigo VARCHAR(30) NOT NULL,
    maquina_nombre VARCHAR(100) NOT NULL,
    material_nombre VARCHAR(100) NOT NULL,
    descripcion VARCHAR(200),
    cantidad_material_gramos NUMERIC(12, 3) NOT NULL,
    duracion_minutos BIGINT NOT NULL,
    cantidad_piezas INTEGER NOT NULL,
    costo_material_por_kg NUMERIC(12, 2) NOT NULL,
    costo_interno_hora NUMERIC(12, 2) NOT NULL,
    precio_venta_hora NUMERIC(12, 2) NOT NULL,
    tarifa_costo_preparacion NUMERIC(12, 2) NOT NULL,
    potencia_watts NUMERIC(10, 2) NOT NULL,
    costo_por_kwh NUMERIC(12, 4) NOT NULL,
    costo_material NUMERIC(14, 2) NOT NULL,
    costo_maquina NUMERIC(14, 2) NOT NULL,
    costo_electrico NUMERIC(14, 2) NOT NULL,
    costo_preparacion NUMERIC(14, 2) NOT NULL,
    subtotal NUMERIC(14, 2) NOT NULL,
    precio_sugerido NUMERIC(14, 2) NOT NULL,
    precio_sugerido_por_pieza NUMERIC(14, 2) NOT NULL,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (cantidad_material_gramos > 0),
    CHECK (duracion_minutos > 0),
    CHECK (cantidad_piezas > 0),
    CHECK (costo_material_por_kg >= 0),
    CHECK (costo_interno_hora >= 0),
    CHECK (precio_venta_hora >= 0),
    CHECK (tarifa_costo_preparacion >= 0),
    CHECK (potencia_watts >= 0),
    CHECK (costo_por_kwh >= 0),
    CHECK (costo_material >= 0),
    CHECK (costo_maquina >= 0),
    CHECK (costo_electrico >= 0),
    CHECK (costo_preparacion >= 0),
    CHECK (subtotal >= 0),
    CHECK (precio_sugerido >= 0),
    CHECK (precio_sugerido_por_pieza >= 0)
);

CREATE INDEX IF NOT EXISTS idx_clientes_nombre ON clientes (nombre);
CREATE INDEX IF NOT EXISTS idx_cotizaciones_cliente_fecha ON cotizaciones (cliente_id, fecha_creacion DESC);
CREATE INDEX IF NOT EXISTS idx_cotizaciones_estado_fecha ON cotizaciones (estado_cotizacion_id, fecha_creacion DESC);
CREATE INDEX IF NOT EXISTS idx_conceptos_cotizacion ON conceptos_cotizacion (cotizacion_id);
