DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pedidos WHERE cotizacion_id IS NULL)
        OR EXISTS (SELECT 1 FROM trabajos WHERE concepto_cotizacion_id IS NULL) THEN
        RAISE EXCEPTION 'No se puede revertir 000005 mientras existan pedidos directos';
    END IF;
END $$;

DROP INDEX idx_pedidos_cliente_fecha;

ALTER TABLE trabajos
    ALTER COLUMN concepto_cotizacion_id SET NOT NULL;

ALTER TABLE pedidos
    DROP CONSTRAINT pedidos_plataforma_check,
    DROP CONSTRAINT pedidos_origen_check,
    ALTER COLUMN cotizacion_id SET NOT NULL,
    DROP COLUMN notas,
    DROP COLUMN plataforma_venta,
    DROP COLUMN origen,
    DROP COLUMN cliente_id;

ALTER TABLE clientes
    DROP CONSTRAINT clientes_tipo_check,
    DROP COLUMN tipo;
