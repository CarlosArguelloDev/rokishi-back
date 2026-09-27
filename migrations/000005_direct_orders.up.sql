ALTER TABLE clientes
    ADD COLUMN tipo VARCHAR(20) NOT NULL DEFAULT 'PERSONA',
    ADD CONSTRAINT clientes_tipo_check CHECK (tipo IN ('PERSONA', 'EMPRESA'));

ALTER TABLE pedidos
    ADD COLUMN cliente_id BIGINT REFERENCES clientes(id),
    ADD COLUMN origen VARCHAR(20) NOT NULL DEFAULT 'CLIENTE',
    ADD COLUMN plataforma_venta VARCHAR(100),
    ADD COLUMN notas TEXT;

UPDATE pedidos p
SET cliente_id = q.cliente_id
FROM cotizaciones q
WHERE q.id = p.cotizacion_id;

ALTER TABLE pedidos
    ALTER COLUMN cliente_id SET NOT NULL,
    ALTER COLUMN cotizacion_id DROP NOT NULL,
    ADD CONSTRAINT pedidos_origen_check CHECK (origen IN ('CLIENTE', 'EMPRESA', 'PLATAFORMA')),
    ADD CONSTRAINT pedidos_plataforma_check CHECK (
        (origen = 'PLATAFORMA' AND plataforma_venta IS NOT NULL AND btrim(plataforma_venta) <> '')
        OR (origen IN ('CLIENTE', 'EMPRESA') AND plataforma_venta IS NULL)
    );

ALTER TABLE trabajos
    ALTER COLUMN concepto_cotizacion_id DROP NOT NULL;

CREATE INDEX idx_pedidos_cliente_fecha ON pedidos (cliente_id, fecha_creacion DESC);
