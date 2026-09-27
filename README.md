# Rokishi API

API REST en Go para gestion de produccion. Incluye salud, catalogos de locaciones y tipos de maquina, y administracion de maquinas.

Para publicar la API con presupuesto limitado y automatizarla desde Jenkins, consulta [DEPLOY_HEROKU_JENKINS.md](DEPLOY_HEROKU_JENKINS.md).

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
$env:CORS_ALLOWED_ORIGINS = 'http://localhost:5173'
```

`DATABASE_URL` es obligatoria. `HTTP_ADDR` y `SHUTDOWN_TIMEOUT` tienen los valores mostrados como predeterminados. Cuando `HTTP_ADDR` no existe, la API acepta `PORT`, que es la variable proporcionada por Heroku. `CORS_ALLOWED_ORIGINS` acepta origenes exactos separados por comas y usa `http://localhost:5173` por defecto. En Heroku debe incluir el dominio de Cloudflare Pages. Usa `sslmode` apropiado para tu servidor. No guardes credenciales reales en archivos del repositorio.

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

## Catalogos de la Fase 2

La API expone:

- `POST /api/locaciones`
- `GET /api/locaciones`
- `GET /api/locaciones/{id}`
- `PATCH /api/locaciones/{id}`
- `POST /api/tipos-maquina`
- `GET /api/tipos-maquina`
- `GET /api/tipos-maquina/{id}`
- `PATCH /api/tipos-maquina/{id}`

Las respuestas exitosas usan `{"data": ...}`. Los errores usan `{"error":{"code":"...","message":"..."}}`. Por ejemplo:

```powershell
Invoke-RestMethod http://localhost:8081/api/locaciones -Method Post -ContentType 'application/json' -Body '{"codigo":"MX-01","nombre":"Taller Centro"}'
Invoke-RestMethod http://localhost:8081/api/locaciones
Invoke-RestMethod http://localhost:8081/api/locaciones/1 -Method Patch -ContentType 'application/json' -Body '{"activa":false}'
```

Los codigos de locacion y los nombres de tipo duplicados devuelven `409`. Los identificadores inexistentes devuelven `404`; los datos invalidos devuelven `422`.

## Maquinas de la Fase 3

La API expone `POST /api/maquinas`, `GET /api/maquinas`, `GET /api/maquinas/{id}` y `PATCH /api/maquinas/{id}`. La lista admite `locacion_id`, `tipo_maquina_id`, `activa` y `q`; este ultimo busca por codigo o nombre.

```powershell
Invoke-RestMethod http://localhost:8081/api/maquinas -Method Post -ContentType 'application/json' -Body '{"locacion_id":1,"tipo_maquina_id":1,"codigo":"IMP-01","nombre":"Prusa MK4"}'
Invoke-RestMethod 'http://localhost:8081/api/maquinas?locacion_id=1&activa=true&q=prusa'
Invoke-RestMethod http://localhost:8081/api/maquinas/1 -Method Patch -ContentType 'application/json' -Body '{"locacion_id":2,"activa":false}'
```

La locacion y el tipo de maquina deben existir. Un codigo duplicado devuelve `409`; referencias o datos invalidos devuelven `422`. Las maquinas se desactivan con `PATCH`, no se eliminan fisicamente. La Fase 3 usa el esquema inicial existente y no requiere una migracion adicional.

## Estados e historial de la Fase 4

La API expone:

- `GET /api/estados-maquina`
- `GET /api/maquinas/{id}/estado-actual`
- `POST /api/maquinas/{id}/cambios-estado`
- `GET /api/maquinas/{id}/historial-estados?desde=...&hasta=...`

El primer cambio abre el primer periodo y devuelve `estado_anterior: null`. Los cambios posteriores cierran el periodo actual y crean el nuevo dentro de una sola transaccion. `fecha_inicio` es opcional y usa la hora del servidor cuando se omite; si se proporciona debe ser RFC3339 y posterior al inicio del estado actual.

```powershell
Invoke-RestMethod http://localhost:8081/api/estados-maquina
Invoke-RestMethod http://localhost:8081/api/maquinas/1/cambios-estado -Method Post -ContentType 'application/json' -Body '{"estado_maquina_id":2,"notas":"Lista para operar"}'
Invoke-RestMethod http://localhost:8081/api/maquinas/1/estado-actual
Invoke-RestMethod 'http://localhost:8081/api/maquinas/1/historial-estados?desde=2026-09-01T00:00:00Z&hasta=2026-10-01T00:00:00Z'
```

El filtro devuelve los periodos que se solapan con el rango solicitado. La migracion `000002_one_open_machine_state` agrega un indice unico parcial que impide tener dos periodos abiertos para la misma maquina. Debe aplicarse antes de desplegar esta version de la API.

## Materiales y tarifas de la Fase 5

Materiales:

- `POST /api/materiales`
- `GET /api/materiales?tipo=...&marca=...&color=...&activo=true`
- `GET /api/materiales/{id}`
- `PATCH /api/materiales/{id}`

Tarifas:

- `GET /api/maquinas/{id}/tarifa`
- `PUT /api/maquinas/{id}/tarifa`
- `GET /api/locaciones/{id}/tarifa-energia`
- `PUT /api/locaciones/{id}/tarifa-energia`

```powershell
Invoke-RestMethod http://localhost:8081/api/materiales -Method Post -ContentType 'application/json' -Body '{"nombre":"PLA negro","tipo":"Filamento","marca":"Polymaker","color":"Negro","costo_por_kg":420.50,"stock_kg":2.500}'
Invoke-RestMethod 'http://localhost:8081/api/materiales?tipo=Filamento&activo=true'
Invoke-RestMethod http://localhost:8081/api/maquinas/1/tarifa -Method Put -ContentType 'application/json' -Body '{"costo_interno_hora":25,"precio_venta_hora":70,"costo_preparacion":15}'
Invoke-RestMethod http://localhost:8081/api/locaciones/1/tarifa-energia -Method Put -ContentType 'application/json' -Body '{"costo_por_kwh":2.3456}'
```

Los importes y existencias deben ser mayores o iguales a cero. `PUT` crea o reemplaza la configuracion de la maquina o locacion indicada sin modificar otras tarifas. Un `GET` devuelve `404` cuando la tarifa aun no se ha configurado. La Fase 5 usa las tablas del esquema inicial y no requiere una migracion nueva.

## Docker

Construye la imagen desde la raiz del repositorio:

```powershell
docker build -t rokishi-api:local .
```

Ejecuta la API publicando el puerto `8081`. En Docker Desktop para Windows, `host.docker.internal` permite conectar el contenedor con PostgreSQL ejecutado en el equipo anfitrion:

```powershell
docker run --rm --name rokishi-api `
  -p 8081:8081 `
  -e HTTP_ADDR=:8081 `
  -e SHUTDOWN_TIMEOUT=10s `
  -e "DATABASE_URL=postgres://usuario:password@host.docker.internal:5432/rokishi?sslmode=disable" `
  rokishi-api:local
```

La imagen ejecuta solamente la API. Las migraciones se aplican por separado con `golang-migrate` antes de iniciar la version correspondiente de la aplicacion.

## Prueba con Heroku

Heroku Postgres agrega `DATABASE_URL` a la configuracion de la aplicacion. La API tambien lee el `PORT` dinamico que Heroku asigna al proceso web, por lo que no debes configurar ninguna de esas dos variables manualmente.

Antes de desplegar la API, aplica la migracion desde tu equipo usando temporalmente la URL administrada por Heroku:

```powershell
$env:DATABASE_URL = heroku config:get DATABASE_URL -a NOMBRE_DE_LA_APP
& "$(go env GOPATH)\bin\migrate.exe" -path .\migrations -database $env:DATABASE_URL up
Remove-Item Env:DATABASE_URL
```

Una vez desplegada, comprueba la API y revisa los registros:

```powershell
Invoke-RestMethod https://NOMBRE_DE_LA_APP.herokuapp.com/api/health
heroku logs --tail -a NOMBRE_DE_LA_APP
```

La respuesta esperada es `{"status":"ok","database":"up"}`. Un `503` indica que la API esta ejecutandose, pero no puede conectarse a PostgreSQL.
