# Despliegue en Heroku con Jenkins

Esta guia despliega la API y PostgreSQL en Heroku mediante el `Dockerfile` del repositorio. Jenkins se ejecuta en tu computadora o en un equipo que ya pagas; no se crea un dyno para Jenkins.

## Presupuesto mensual

Precios consultados el 24 de septiembre de 2026:

| Recurso | Plan | Costo maximo |
|---|---|---:|
| API | Eco | USD 5/mes |
| PostgreSQL | Essential-0 | USD 5/mes |
| Total recomendado para comenzar | | **USD 10/mes** |

El dyno Eco incluye 1,000 horas compartidas entre tus aplicaciones y duerme despues de 30 minutos sin trafico. La primera peticion despues de dormir tarda unos segundos adicionales.

Cuando necesites que la API permanezca encendida, cambia el dyno a Basic. Basic cuesta hasta USD 7/mes y Essential-0 USD 5/mes: **USD 12/mes antes de impuestos**. Con un limite estricto de USD 13, Eco deja mas margen para impuestos o variaciones. No agregues Redis, monitoreo de pago, otra base ni aplicaciones de prueba permanentes.

Fuentes oficiales: [precios de Heroku](https://www.heroku.com/pricing/), [horas Eco](https://devcenter.heroku.com/articles/eco-dyno-hours) y [facturacion](https://devcenter.heroku.com/articles/usage-and-billing).

## 1. Requisitos del agente Jenkins

El equipo que ejecuta Jenkins necesita estas herramientas en su `PATH`:

- Git.
- Go 1.26 o posterior.
- Docker Desktop o Docker Engine en ejecucion.
- Heroku CLI.
- `golang-migrate` con soporte para PostgreSQL.

Instala `golang-migrate` desde PowerShell:

```powershell
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Verifica todo usando la misma cuenta de Windows que ejecuta el agente Jenkins:

```powershell
git --version
go version
docker version
heroku version
migrate -version
```

Si Jenkins funciona como servicio de Windows, su cuenta debe tener permiso para utilizar Docker. Una prueba en tu terminal personal no demuestra que el servicio Jenkins tenga ese acceso.

## 2. Crear la aplicacion y la base

Inicia sesion y elige un nombre globalmente unico. Sustituye `NOMBRE_DE_LA_APP` en todos los comandos:

```powershell
heroku login
heroku create NOMBRE_DE_LA_APP --stack container
heroku addons:create heroku-postgresql:essential-0 --app NOMBRE_DE_LA_APP --wait
heroku pg:info --app NOMBRE_DE_LA_APP
```

Heroku crea y administra `DATABASE_URL`. No copies esa URL al repositorio ni la configures manualmente en la aplicacion. La API ya lee `DATABASE_URL` y el `PORT` dinamico de Heroku.

En una cuenta personal suscrita a Eco, las aplicaciones nuevas usan Eco de forma predeterminada. Confirma el tipo y que exista un solo proceso web desde **Heroku Dashboard > App > Resources** despues del primer despliegue.

## 3. Probar el primer despliegue manual

Conviene completar un despliegue manual antes de configurar Jenkins. Asi se comprueban Docker, Heroku y la base por separado.

```powershell
heroku container:login
heroku container:push web --app NOMBRE_DE_LA_APP

$env:DATABASE_URL = heroku config:get DATABASE_URL --app NOMBRE_DE_LA_APP
migrate -path .\migrations -database $env:DATABASE_URL up
Remove-Item Env:DATABASE_URL

heroku container:release web --app NOMBRE_DE_LA_APP
heroku ps:scale web=1 --app NOMBRE_DE_LA_APP
```

Comprueba el resultado:

```powershell
Invoke-RestMethod https://NOMBRE_DE_LA_APP.herokuapp.com/api/health
heroku logs --tail --app NOMBRE_DE_LA_APP
```

La respuesta correcta es:

```json
{"status":"ok","database":"up"}
```

Un `503` significa que el contenedor arranco, pero no puede conectarse a PostgreSQL. Un error `H10` normalmente significa que el proceso no arranco o no escucho en el puerto asignado; revisa los logs.

## 4. Crear el token de Jenkins

Genera una autorizacion dedicada y copia el valor de `Token` una sola vez:

```powershell
heroku authorizations:create --description "Jenkins rokishi-back"
```

En Jenkins abre **Manage Jenkins > Credentials > System > Global credentials** y crea dos credenciales de tipo **Secret text**:

| ID | Secret |
|---|---|
| `heroku-api-key` | El token generado por Heroku |
| `heroku-app-name` | El nombre exacto de la aplicacion |

No escribas el token en el `Jenkinsfile`, en variables globales visibles ni en Git. Heroku CLI acepta `HEROKU_API_KEY`, y Jenkins la inyecta solamente durante las etapas que la necesitan.

## 5. Crear el trabajo en Jenkins

1. Crea un elemento de tipo **Multibranch Pipeline**.
2. Agrega GitHub como origen y selecciona el repositorio `rokishi-back`.
3. Agrega credenciales de GitHub si el repositorio es privado.
4. Usa `Jenkinsfile` como ruta del script.
5. Ejecuta **Scan Multibranch Pipeline Now**.

El pipeline hace lo siguiente:

1. Descarga el commit.
2. Ejecuta `go test ./...`.
3. Construye la imagen Docker.
4. En `main`, aplica las migraciones pendientes.
5. Publica y libera la imagen en Heroku.
6. Reintenta `/api/health` hasta confirmar API y base.

El `Jenkinsfile` consulta GitHub cada cinco minutos con `pollSCM`. Esto evita exponer tu Jenkins local a Internet. Si mas adelante Jenkins tiene HTTPS publico, puedes sustituirlo por un webhook de GitHub.

## 6. Flujo diario

Trabaja en una rama y subela a GitHub:

```powershell
git checkout -b feature/mi-cambio
git add .
git commit -m "Describe el cambio"
git push -u origin feature/mi-cambio
```

Jenkins ejecutara pruebas y construira la imagen para la rama. El despliegue y las migraciones ocurren solamente cuando el cambio llega a `main`.

## 7. Control del gasto

Revisa **Heroku Dashboard > Account Settings > Billing** cada pocos dias durante el primer mes. El uso mostrado puede llevar retraso.

```powershell
heroku addons --app NOMBRE_DE_LA_APP
heroku ps --app NOMBRE_DE_LA_APP
```

Debe existir una base Essential-0 y un solo dyno `web`. No habilites Heroku CI ni Review Apps: ya utilizas Jenkins y esos entornos pueden consumir horas o crear complementos adicionales.

Para detener temporalmente la API sin borrar la base:

```powershell
heroku ps:scale web=0 --app NOMBRE_DE_LA_APP
```

## 8. Problemas comunes

- **`docker` no se reconoce:** instala Docker y reinicia el agente Jenkins.
- **Jenkins no accede a Docker:** ejecuta el agente con una cuenta autorizada y confirma `docker version` desde un trabajo.
- **`migrate` no se reconoce:** agrega la carpeta devuelta por `go env GOPATH`, seguida de `\bin`, al `PATH` del agente.
- **Falla el login al registro:** reemplaza el secreto `heroku-api-key` con un token vigente.
- **La migracion queda en estado dirty:** no uses `force` a ciegas; revisa primero la migracion fallida y la base.
- **Health devuelve `503`:** ejecuta `heroku pg:info` y revisa `heroku logs --tail`.
- **La primera peticion tarda:** es normal cuando un dyno Eco despierta.

## Referencias

- [Heroku Container Registry](https://devcenter.heroku.com/articles/container-registry-and-runtime)
- [Provisionar Heroku Postgres](https://devcenter.heroku.com/articles/provisioning-heroku-postgres)
- [Autenticacion de Heroku CLI](https://devcenter.heroku.com/articles/authentication)
