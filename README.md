# Rokishi API

API REST en Go para gestion de produccion. Esta primera fase expone `GET /api/health` y comprueba PostgreSQL mediante el pool de conexiones.

## Requisitos

- Go 1.26 o posterior.
- PostgreSQL y una base de datos vacia.
- CLI de `golang-migrate` para aplicar migraciones. En Windows se puede instalar con `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`. La instalacion queda a cargo del desarrollador.

## Configuracion local (PowerShell)

Desde este directorio, configura las variables para tu entorno. La API lee variables de entorno directamente; `.env.example` es una referencia y no se carga de forma automatica.

```powershell
$env:DATABASE_URL = 'postgres://usuario:password@localhost:5432/rokishi?sslmode=disable'
$env:HTTP_ADDR = ':8081'
$env:SHUTDOWN_TIMEOUT = '10s'
```

`DATABASE_URL` es obligatoria. `HTTP_ADDR` y `SHUTDOWN_TIMEOUT` tienen los valores mostrados como predeterminados. Usa `sslmode` apropiado para tu servidor. No guardes credenciales reales en archivos del repositorio.

Aplica el esquema inicial **solo en una base vacia**. Si ya cargaste `gestor_impresiones_3d_basico.sql` en esa base, crea otra para esta migracion: el `down` elimina las tablas, incluidos sus datos.

```powershell
& "$(go env GOPATH)\bin\migrate.exe" -path .\migrations -database $env:DATABASE_URL up
go run ./cmd/api
```

En otra terminal:

```powershell
Invoke-RestMethod http://localhost:8081/api/health
```

Con PostgreSQL disponible responde `200` y `{"status":"ok","database":"up"}`. Si PostgreSQL no responde, devuelve `503` con `database: "down"` y un error JSON `database_unavailable`. Una ruta desconocida devuelve `404` y un metodo no permitido devuelve `405`, ambos en JSON. La API puede iniciar sin PostgreSQL disponible para que salud informe la falla; una configuracion invalida impide el inicio.

Para ejecutar pruebas:

```powershell
go test ./...
```
