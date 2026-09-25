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

## 1. Preparar Jenkins en Docker

Conectate por SSH a la Raspberry y confirma su arquitectura:

```bash
uname -m
```

Lo habitual es `aarch64` o `arm64`. Heroku Container Registry solo ejecuta imagenes `x86_64`, por lo que el pipeline construye explicitamente para `linux/amd64`.

El contenedor oficial de Jenkins no incluye Docker CLI. Este repositorio contiene `deploy/jenkins/Dockerfile`, una imagen personalizada con Git, Go, Docker CLI/Buildx, Heroku CLI, `golang-migrate` y `curl`.

### 1.1 Identificar el almacenamiento actual

Antes de reemplazar el contenedor, identifica su nombre y el volumen o directorio montado en `/var/jenkins_home`:

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}'
docker inspect NOMBRE_JENKINS_ACTUAL \
  --format '{{range .Mounts}}{{println .Type .Name .Source .Destination}}{{end}}'
```

Anota exactamente el origen asociado con `/var/jenkins_home`. Ese almacenamiento contiene trabajos, plugins y credenciales. Reutilizarlo conserva tu Jenkins actual. No elimines el contenedor ni el volumen existentes antes de verificar la nueva instancia.

### 1.2 Construir la imagen de Jenkins

Desde una copia de este repositorio en la Raspberry:

```bash
docker build --tag rokishi-jenkins:lts deploy/jenkins
```

La imagen se construye para la arquitectura de la Raspberry. Solamente las imagenes de la API se generan como `linux/amd64` para Heroku.

### 1.3 Crear el daemon Docker para Jenkins

La configuracion recomendada por Jenkins usa un contenedor Docker-in-Docker separado y comunicacion TLS:

```bash
docker network create jenkins
docker volume create jenkins-docker-certs
docker volume create jenkins-docker-data

docker run --name jenkins-docker \
  --detach \
  --restart unless-stopped \
  --privileged \
  --network jenkins \
  --network-alias docker \
  --env DOCKER_TLS_CERTDIR=/certs \
  --volume jenkins-docker-certs:/certs/client \
  --volume jenkins-docker-data:/var/lib/docker \
  docker:dind \
  --storage-driver overlay2
```

Si esos nombres ya existen porque Jenkins ya usa Docker-in-Docker, no crees duplicados: inspecciona y reutiliza la configuracion actual.

### 1.4 Reemplazar el contenedor sin perder Jenkins

Deten el contenedor actual y conservalo como respaldo:

```bash
docker stop NOMBRE_JENKINS_ACTUAL
docker rename NOMBRE_JENKINS_ACTUAL jenkins-backup
```

Inicia la imagen personalizada reutilizando **el mismo origen** que encontraste para `/var/jenkins_home`. Si era un volumen llamado `jenkins_home`, el comando es:

```bash
docker run --name jenkins \
  --detach \
  --restart unless-stopped \
  --network jenkins \
  --env DOCKER_HOST=tcp://docker:2376 \
  --env DOCKER_CERT_PATH=/certs/client \
  --env DOCKER_TLS_VERIFY=1 \
  --volume jenkins_home:/var/jenkins_home \
  --volume jenkins-docker-certs:/certs/client:ro \
  --publish 8080:8080 \
  --publish 50000:50000 \
  rokishi-jenkins:lts
```

Si usabas un directorio del host, sustituye `jenkins_home` por la ruta exacta, por ejemplo `/srv/jenkins:/var/jenkins_home`. No uses un volumen nuevo por accidente: Jenkins apareceria vacio aunque tus datos anteriores siguieran almacenados en otro volumen.

Prepara Buildx y verifica todas las herramientas dentro del contenedor:

```bash
docker exec jenkins docker buildx create \
  --name jenkins-builder \
  --driver docker-container \
  --use
docker exec jenkins docker buildx inspect --bootstrap

docker exec jenkins git --version
docker exec jenkins go version
docker exec jenkins docker version
docker exec jenkins heroku version
docker exec jenkins migrate -version
docker exec jenkins curl --version
```

Abre Jenkins en el puerto `8080` y confirma que tus trabajos y credenciales siguen presentes. Conserva `jenkins-backup` hasta completar varios builds correctamente.

## 2. Crear la aplicacion y la base

Abre una terminal dentro del contenedor personalizado, inicia sesion y elige un nombre globalmente unico. Sustituye `NOMBRE_DE_LA_APP` en todos los comandos:

```bash
docker exec -it jenkins bash
heroku login
heroku create NOMBRE_DE_LA_APP --stack container
heroku addons:create heroku-postgresql:essential-0 --app NOMBRE_DE_LA_APP --wait
heroku pg:info --app NOMBRE_DE_LA_APP
heroku authorizations:create --description "Jenkins rokishi-back"
```

Heroku crea y administra `DATABASE_URL`. No copies esa URL al repositorio ni la configures manualmente en la aplicacion. La API ya lee `DATABASE_URL` y el `PORT` dinamico de Heroku.

Copia el valor `Token` producido por `authorizations:create`; se guardara en Jenkins en el siguiente paso. Es distinto de tu contrasena de Heroku.

En una cuenta personal suscrita a Eco, las aplicaciones nuevas usan Eco de forma predeterminada. Confirma el tipo y que exista un solo proceso web desde **Heroku Dashboard > App > Resources** despues del primer despliegue.

## 3. Configurar el despliegue en Jenkins

En Jenkins abre **Manage Jenkins > Credentials > System > Global credentials** y crea dos credenciales de tipo **Secret text**:

| ID | Secret |
|---|---|
| `heroku-api-key` | El token generado por Heroku |
| `heroku-app-name` | El nombre exacto de la aplicacion |

No escribas el token en el `Jenkinsfile`, en variables globales visibles ni en Git. Heroku CLI acepta `HEROKU_API_KEY`, y Jenkins la inyecta solamente durante las etapas que la necesitan.

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

Fusiona o sube el proyecto a `main` y ejecuta **Build Now**. El primer pipeline aplicara la migracion inicial, publicara la API y comprobara la base.

Comprueba tambien desde la Raspberry:

```bash
curl --fail --show-error https://NOMBRE_DE_LA_APP.herokuapp.com/api/health
docker exec jenkins heroku logs --tail --app NOMBRE_DE_LA_APP
```

La respuesta correcta es `{"status":"ok","database":"up"}`. Un `503` significa que el contenedor de la API arranco, pero no puede conectarse a PostgreSQL. Un error `H10` normalmente significa que el proceso no arranco o no escucho en el puerto asignado.

El pipeline elimina las etiquetas de imagen que crea al terminar. Docker conserva una cache de compilacion para acelerar ejecuciones posteriores. Revisa periodicamente su consumo:

```bash
docker exec jenkins docker system df
```

Si necesitas recuperar espacio, limpia cache de compilacion con mas de siete dias. Este comando puede hacer mas lenta la siguiente compilacion y afecta a todos los proyectos Docker de la Raspberry:

```bash
docker exec jenkins docker builder prune --filter until=168h --force
```

## 4. Flujo diario

Trabaja en una rama y subela a GitHub:

```bash
git checkout -b feature/mi-cambio
git add .
git commit -m "Describe el cambio"
git push -u origin feature/mi-cambio
```

Jenkins ejecutara pruebas y construira la imagen para la rama. El despliegue y las migraciones ocurren solamente cuando el cambio llega a `main`.

## 5. Control del gasto

Revisa **Heroku Dashboard > Account Settings > Billing** cada pocos dias durante el primer mes. El uso mostrado puede llevar retraso.

```bash
docker exec jenkins heroku addons --app NOMBRE_DE_LA_APP
docker exec jenkins heroku ps --app NOMBRE_DE_LA_APP
```

Debe existir una base Essential-0 y un solo dyno `web`. No habilites Heroku CI ni Review Apps: ya utilizas Jenkins y esos entornos pueden consumir horas o crear complementos adicionales.

Para detener temporalmente la API sin borrar la base:

```bash
docker exec jenkins heroku ps:scale web=0 --app NOMBRE_DE_LA_APP
```

## 6. Problemas comunes

- **`docker` no existe dentro de Jenkins:** confirma que el contenedor usa la imagen `rokishi-jenkins:lts`.
- **Jenkins no conecta con Docker:** revisa que `jenkins-docker` este activo, ambos contenedores usen la red `jenkins`, el volumen de certificados este montado y las variables `DOCKER_HOST`, `DOCKER_CERT_PATH` y `DOCKER_TLS_VERIFY` existan.
- **`docker buildx` no existe:** reconstruye `rokishi-jenkins:lts` y vuelve a crear el contenedor con esa imagen.
- **Heroku rechaza la arquitectura:** confirma que el build usa `--platform linux/amd64`; Heroku Container Registry no acepta ARM64.
- **`migrate` no existe:** reconstruye la imagen personalizada; el binario debe estar en `/usr/local/bin/migrate`.
- **Falla el login al registro:** reemplaza el secreto `heroku-api-key` con un token vigente.
- **La migracion queda en estado dirty:** no uses `force` a ciegas; revisa primero la migracion fallida y la base.
- **Health devuelve `503`:** ejecuta `docker exec jenkins heroku pg:info --app NOMBRE_DE_LA_APP` y revisa los logs.
- **La primera peticion tarda:** es normal cuando un dyno Eco despierta.

## Referencias

- [Heroku Container Registry](https://devcenter.heroku.com/articles/container-registry-and-runtime)
- [Provisionar Heroku Postgres](https://devcenter.heroku.com/articles/provisioning-heroku-postgres)
- [Autenticacion de Heroku CLI](https://devcenter.heroku.com/articles/authentication)
- [Instalar Docker Engine en Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- [Jenkins en Docker](https://www.jenkins.io/doc/book/installing/docker/)
