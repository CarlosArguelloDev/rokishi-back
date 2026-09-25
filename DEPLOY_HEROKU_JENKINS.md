# Despliegue en Heroku con Jenkins

Esta guia despliega la API y PostgreSQL en Heroku mediante el `Dockerfile` del repositorio. Jenkins se ejecuta en una Raspberry Pi con Ubuntu; no se crea un dyno para Jenkins.

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

## 1. Preparar la Raspberry Pi

Conectate por SSH a la Raspberry y confirma su arquitectura:

```bash
uname -m
```

Lo habitual es `aarch64` o `arm64`. Heroku Container Registry solo ejecuta imagenes `x86_64`, por lo que el pipeline construye explicitamente para `linux/amd64`. El `Dockerfile` compila el binario amd64 desde el compilador nativo de la Raspberry y evita ejecutar capas amd64 durante la construccion.

El agente Jenkins necesita estas herramientas:

- Git.
- Go 1.26 o posterior.
- Docker Engine con `buildx`.
- Heroku CLI.
- `golang-migrate` con soporte para PostgreSQL.
- `curl`.

Instala Docker Engine usando la [guia oficial para Ubuntu](https://docs.docker.com/engine/install/ubuntu/). Luego permite que Jenkins use Docker y reinicia el servicio:

```bash
sudo usermod -aG docker jenkins
sudo systemctl restart jenkins
```

Instala Heroku CLI para Ubuntu. El instalador selecciona la compilacion ARM adecuada:

```bash
curl https://cli-assets.heroku.com/install-ubuntu.sh | sh
```

Instala `golang-migrate` en el directorio de la cuenta Jenkins:

```bash
sudo -u jenkins -H bash -lc "go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
```

Verifica las herramientas **como el usuario Jenkins**, no solamente desde tu usuario SSH:

```bash
sudo -u jenkins -H git --version
sudo -u jenkins -H go version
sudo -u jenkins -H docker version
sudo -u jenkins -H docker buildx version
sudo -u jenkins -H heroku version
sudo -u jenkins -H /var/lib/jenkins/go/bin/migrate -version
sudo -u jenkins -H curl --version
```

Si el `HOME` real de Jenkins no es `/var/lib/jenkins`, consultalo con `getent passwd jenkins` y utiliza la ruta indicada. Ajusta tambien la llamada a `migrate` en el `Jenkinsfile`.

## 2. Crear la aplicacion y la base

Inicia sesion y elige un nombre globalmente unico. Sustituye `NOMBRE_DE_LA_APP` en todos los comandos:

```bash
heroku login
heroku create NOMBRE_DE_LA_APP --stack container
heroku addons:create heroku-postgresql:essential-0 --app NOMBRE_DE_LA_APP --wait
heroku pg:info --app NOMBRE_DE_LA_APP
```

Heroku crea y administra `DATABASE_URL`. No copies esa URL al repositorio ni la configures manualmente en la aplicacion. La API ya lee `DATABASE_URL` y el `PORT` dinamico de Heroku.

En una cuenta personal suscrita a Eco, las aplicaciones nuevas usan Eco de forma predeterminada. Confirma el tipo y que exista un solo proceso web desde **Heroku Dashboard > App > Resources** despues del primer despliegue.

## 3. Probar el primer despliegue manual

Conviene completar un despliegue manual antes de configurar Jenkins. Asi se comprueban Docker, Heroku y la base por separado.

Instala tambien `migrate` para tu usuario SSH si todavia no existe en `$HOME/go/bin`:

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

```bash
docker buildx build \
  --platform linux/amd64 \
  --pull \
  --load \
  --tag rokishi-api:manual \
  .

heroku container:login
docker tag rokishi-api:manual registry.heroku.com/NOMBRE_DE_LA_APP/web
docker push registry.heroku.com/NOMBRE_DE_LA_APP/web

database_url="$(heroku config:get DATABASE_URL --app NOMBRE_DE_LA_APP)"
$HOME/go/bin/migrate -path ./migrations -database "$database_url" up
unset database_url

heroku container:release web --app NOMBRE_DE_LA_APP
heroku ps:scale web=1 --app NOMBRE_DE_LA_APP
```

Comprueba el resultado:

```bash
curl --fail --show-error https://NOMBRE_DE_LA_APP.herokuapp.com/api/health
heroku logs --tail --app NOMBRE_DE_LA_APP
```

La respuesta correcta es:

```json
{"status":"ok","database":"up"}
```

Un `503` significa que el contenedor arranco, pero no puede conectarse a PostgreSQL. Un error `H10` normalmente significa que el proceso no arranco o no escucho en el puerto asignado; revisa los logs.

## 4. Crear el token de Jenkins

Genera una autorizacion dedicada y copia el valor de `Token` una sola vez:

```bash
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
3. Construye una imagen `linux/amd64`, aunque Jenkins se ejecute en ARM64.
4. En `main`, aplica las migraciones pendientes.
5. Publica y libera la imagen en Heroku.
6. Reintenta `/api/health` hasta confirmar API y base.

El `Jenkinsfile` consulta GitHub cada cinco minutos con `pollSCM`. Esto evita exponer tu Jenkins local a Internet. Si mas adelante Jenkins tiene HTTPS publico, puedes sustituirlo por un webhook de GitHub.

El pipeline elimina las etiquetas de imagen que crea al terminar. Docker conserva una cache de compilacion para acelerar ejecuciones posteriores. Revisa periodicamente su consumo:

```bash
docker system df
```

Si necesitas recuperar espacio, limpia cache de compilacion con mas de siete dias. Este comando puede hacer mas lenta la siguiente compilacion y afecta a todos los proyectos Docker de la Raspberry:

```bash
docker builder prune --filter until=168h --force
```

## 6. Flujo diario

Trabaja en una rama y subela a GitHub:

```bash
git checkout -b feature/mi-cambio
git add .
git commit -m "Describe el cambio"
git push -u origin feature/mi-cambio
```

Jenkins ejecutara pruebas y construira la imagen para la rama. El despliegue y las migraciones ocurren solamente cuando el cambio llega a `main`.

## 7. Control del gasto

Revisa **Heroku Dashboard > Account Settings > Billing** cada pocos dias durante el primer mes. El uso mostrado puede llevar retraso.

```bash
heroku addons --app NOMBRE_DE_LA_APP
heroku ps --app NOMBRE_DE_LA_APP
```

Debe existir una base Essential-0 y un solo dyno `web`. No habilites Heroku CI ni Review Apps: ya utilizas Jenkins y esos entornos pueden consumir horas o crear complementos adicionales.

Para detener temporalmente la API sin borrar la base:

```bash
heroku ps:scale web=0 --app NOMBRE_DE_LA_APP
```

## 8. Problemas comunes

- **`docker` no se reconoce:** instala Docker Engine y reinicia Jenkins.
- **Jenkins recibe `permission denied` con Docker:** confirma que `jenkins` pertenece al grupo `docker` y reinicia Jenkins.
- **`docker buildx` no existe:** instala el complemento Buildx para Docker Engine.
- **Heroku rechaza la arquitectura:** confirma que el build usa `--platform linux/amd64`; Heroku Container Registry no acepta ARM64.
- **`migrate` no existe:** confirma la ruta con `sudo -u jenkins -H sh -c 'go env GOPATH'` y actualiza el `Jenkinsfile` si no es `/var/lib/jenkins/go`.
- **Falla el login al registro:** reemplaza el secreto `heroku-api-key` con un token vigente.
- **La migracion queda en estado dirty:** no uses `force` a ciegas; revisa primero la migracion fallida y la base.
- **Health devuelve `503`:** ejecuta `heroku pg:info` y revisa `heroku logs --tail`.
- **La primera peticion tarda:** es normal cuando un dyno Eco despierta.

## Referencias

- [Heroku Container Registry](https://devcenter.heroku.com/articles/container-registry-and-runtime)
- [Provisionar Heroku Postgres](https://devcenter.heroku.com/articles/provisioning-heroku-postgres)
- [Autenticacion de Heroku CLI](https://devcenter.heroku.com/articles/authentication)
- [Instalar Docker Engine en Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
