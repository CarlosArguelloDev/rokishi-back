CREATE TABLE usuarios (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    correo VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,
    rol VARCHAR(20) NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    es_usuario_inicial BOOLEAN NOT NULL DEFAULT FALSE,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fecha_actualizacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (rol IN ('ADMIN', 'OPERADOR')),
    CHECK (btrim(nombre) <> ''),
    CHECK (btrim(correo) <> '')
);

CREATE UNIQUE INDEX uq_usuarios_correo_normalizado ON usuarios (lower(correo));
CREATE UNIQUE INDEX uq_usuarios_inicial ON usuarios (es_usuario_inicial) WHERE es_usuario_inicial;

CREATE TABLE sesiones (
    token_hash CHAR(64) PRIMARY KEY,
    usuario_id BIGINT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    fecha_expiracion TIMESTAMPTZ NOT NULL,
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ultima_actividad TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sesiones_usuario ON sesiones (usuario_id);
CREATE INDEX idx_sesiones_expiracion ON sesiones (fecha_expiracion);

CREATE TABLE auditoria (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id BIGINT REFERENCES usuarios(id) ON DELETE SET NULL,
    usuario_nombre VARCHAR(100) NOT NULL,
    accion VARCHAR(10) NOT NULL,
    recurso VARCHAR(200) NOT NULL,
    estado_http INTEGER NOT NULL,
    direccion_ip VARCHAR(100),
    agente_usuario VARCHAR(500),
    fecha TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_auditoria_fecha ON auditoria (fecha DESC);
CREATE INDEX idx_auditoria_usuario_fecha ON auditoria (usuario_id, fecha DESC);
