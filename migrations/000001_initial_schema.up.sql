CREATE TABLE IF NOT EXISTS locaciones (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(20) NOT NULL UNIQUE,
    nombre VARCHAR(100) NOT NULL,
    direccion TEXT,
    ciudad VARCHAR(100),
    estado VARCHAR(100),
    zona_horaria VARCHAR(50) NOT NULL DEFAULT 'America/Mexico_City',
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tipos_maquina (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL UNIQUE,
    descripcion TEXT
);

CREATE TABLE IF NOT EXISTS maquinas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    locacion_id BIGINT NOT NULL REFERENCES locaciones(id),
    tipo_maquina_id BIGINT NOT NULL REFERENCES tipos_maquina(id),
    codigo VARCHAR(30) NOT NULL UNIQUE,
    nombre VARCHAR(100) NOT NULL,
    marca VARCHAR(100),
    modelo VARCHAR(100),
    numero_serie VARCHAR(100),
    potencia_watts NUMERIC(10, 2),
    fecha_compra DATE,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (potencia_watts IS NULL OR potencia_watts >= 0)
);

CREATE TABLE IF NOT EXISTS estados_maquina (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(30) NOT NULL UNIQUE,
    nombre VARCHAR(60) NOT NULL,
    descripcion TEXT
);

INSERT INTO estados_maquina (codigo, nombre, descripcion)
VALUES
    ('TRABAJANDO', 'Trabajando', 'La maquina esta realizando un trabajo'),
    ('DISPONIBLE', 'Disponible', 'Esta encendida y lista, pero sin trabajo'),
    ('APAGADA', 'Apagada', 'La maquina esta apagada'),
    ('MANTENIMIENTO', 'Mantenimiento', 'No esta disponible por mantenimiento'),
    ('FALLA', 'Falla', 'No esta disponible debido a una averia'),
    ('PAUSADA', 'Pausada', 'El trabajo fue pausado temporalmente')
ON CONFLICT (codigo) DO NOTHING;

CREATE TABLE IF NOT EXISTS historial_estados_maquina (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    maquina_id BIGINT NOT NULL REFERENCES maquinas(id),
    estado_maquina_id BIGINT NOT NULL REFERENCES estados_maquina(id),
    fecha_inicio TIMESTAMPTZ NOT NULL,
    fecha_fin TIMESTAMPTZ,
    notas TEXT,
    CHECK (fecha_fin IS NULL OR fecha_fin > fecha_inicio)
);

CREATE TABLE IF NOT EXISTS materiales (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    tipo VARCHAR(50) NOT NULL,
    marca VARCHAR(100),
    color VARCHAR(50),
    costo_por_kg NUMERIC(12, 2) NOT NULL,
    stock_kg NUMERIC(12, 3) NOT NULL DEFAULT 0,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (costo_por_kg >= 0),
    CHECK (stock_kg >= 0)
);

CREATE TABLE IF NOT EXISTS tarifas_maquina (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    maquina_id BIGINT NOT NULL UNIQUE REFERENCES maquinas(id),
    costo_interno_hora NUMERIC(12, 2) NOT NULL DEFAULT 0,
    precio_venta_hora NUMERIC(12, 2) NOT NULL DEFAULT 0,
    costo_preparacion NUMERIC(12, 2) NOT NULL DEFAULT 0,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (costo_interno_hora >= 0),
    CHECK (precio_venta_hora >= 0),
    CHECK (costo_preparacion >= 0)
);

CREATE TABLE IF NOT EXISTS tarifas_energia (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    locacion_id BIGINT NOT NULL UNIQUE REFERENCES locaciones(id),
    costo_por_kwh NUMERIC(12, 4) NOT NULL,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (costo_por_kwh >= 0)
);

CREATE INDEX IF NOT EXISTS idx_maquinas_locacion ON maquinas (locacion_id);
CREATE INDEX IF NOT EXISTS idx_maquinas_tipo ON maquinas (tipo_maquina_id);
CREATE INDEX IF NOT EXISTS idx_historial_maquina_fecha ON historial_estados_maquina (maquina_id, fecha_inicio);
