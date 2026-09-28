# Heroku CI/CD con Jenkins en Raspberry Pi

Esta guia corresponde a esta instalacion concreta:

- Raspberry Pi con Ubuntu y Docker Engine.
- Jenkins ejecutado como contenedor.
- Datos persistentes en `/home/charli/jenkins-lab/jenkins_home`.
- Jenkins usa el Docker del host mediante `/var/run/docker.sock`.
- Cloudflared publica Jenkins mediante un tunel administrado remotamente.
- La API y PostgreSQL se ejecutaran en Heroku.

## 0. Rotar primero el token de Cloudflare

El token del tunel fue expuesto y debe considerarse comprometido. Cualquier persona que tenga ese valor puede ejecutar un conector para el tunel.

1. Abre **Cloudflare Dashboard > Networking > Tunnels**.
2. Selecciona el tunel usado por Jenkins.
3. Selecciona **Rotate token** o **Refresh token**.
4. Copia el nuevo token, pero no lo pegues en Git, Jenkinsfile, Compose ni mensajes.
5. Tras actualizar Cloudflared, fuerza la desconexion de conectores anteriores desde Cloudflare.

Cloudflare documenta este procedimiento en [Tunnel tokens](https://developers.cloudflare.com/tunnel/reference/tunnel-tokens/).

## 1. Presupuesto de Heroku

Precios consultados el 26 de septiembre de 2026:

| Recurso | Plan | Costo maximo |
|---|---|---:|
| API | Eco | USD 5/mes |
| PostgreSQL | Essential-0 | USD 5/mes |
| Total recomendado | | **USD 10/mes** |

Eco duerme despues de 30 minutos sin trafico. Si necesitas una API siempre activa, Basic cuesta hasta USD 7 y lleva el total a USD 12 antes de impuestos. Con un limite estricto de USD 13, comienza con Eco.

No habilites Heroku CI ni Review Apps porque Jenkins ya cubre CI/CD y esos entornos pueden consumir recursos adicionales.

Fuentes: [precios de Heroku](https://www.heroku.com/pricing/), [Eco Dynos](https://devcenter.heroku.com/articles/eco-dyno-hours) y [facturacion](https://devcenter.heroku.com/articles/usage-and-billing).

## 2. Que cambia en Jenkins

El contenedor actual monta `/usr/bin/docker` desde Ubuntu. Ese montaje puede fallar cuando la biblioteca del host no coincide con la distribucion dentro del contenedor.

La configuracion nueva:

- Conserva `./jenkins_home:/var/jenkins_home`.
- Conserva `/var/run/docker.sock:/var/run/docker.sock`.
- Elimina el montaje `/usr/bin/docker:/usr/bin/docker`.
- Construye una imagen Jenkins con Docker CLI, Buildx, Go, Heroku CLI, `migrate`, Git y `curl`.
- Actualiza Jenkins de `lts-jdk17` a `lts-jdk21`, requerido por las LTS actuales.
- Mantiene Cloudflared como segundo servicio.
- Lee el token de Cloudflare desde un archivo secreto montado en `/run/secrets`.

El socket de Docker proporciona a los trabajos de Jenkins control equivalente a `root` sobre la Raspberry. Permite ejecutar pipelines unicamente desde repositorios y usuarios de confianza. Protege el hostname publico de Jenkins con Cloudflare Access, ademas del inicio de sesion de Jenkins.

## 3. Preparar los archivos en la Raspberry

Conectate por SSH y entra al directorio actual:

```bash
cd /home/charli/jenkins-lab
```

Desde una copia actualizada de `rokishi-back`, copia los archivos preparados. Sustituye `/RUTA/AL/REPO`:

```bash
cp /RUTA/AL/REPO/deploy/jenkins/Dockerfile ./Dockerfile.jenkins
cp /RUTA/AL/REPO/deploy/jenkins/docker-compose.raspberry.yml ./docker-compose.yml.new
cp /RUTA/AL/REPO/deploy/jenkins/dockerignore.raspberry ./.dockerignore
```

Construye la nueva imagen antes de detener Jenkins:

```bash
docker build --tag rokishi-jenkins:lts --file Dockerfile.jenkins .
```

La imagen se construye para ARM64, que es la arquitectura de la Raspberry. El pipeline genera por separado la API para `linux/amd64`, porque Heroku Container Registry solo acepta `x86_64`.

## 4. Detener y respaldar Jenkins

Guarda el Compose actual y detiene los servicios:

```bash
cp docker-compose.yml docker-compose.yml.before-heroku
docker compose down
```

`docker compose down` no borra `./jenkins_home`. Crea de todas formas un respaldo antes de continuar:

```bash
sudo tar -C /home/charli/jenkins-lab \
  -czf "/home/charli/jenkins-home-$(date +%Y%m%d-%H%M%S).tar.gz" \
  jenkins_home
```

No uses `docker compose down -v` y no elimines `jenkins_home`.

## 5. Guardar el nuevo token como secreto

Crea el archivo secreto sin escribir el token en el historial del shell:

```bash
cd /home/charli/jenkins-lab
umask 077
mkdir -p secrets
read -s -p "Nuevo token de Cloudflare: " CLOUDFLARE_TUNNEL_TOKEN
printf '\n'
printf '%s' "$CLOUDFLARE_TUNNEL_TOKEN" > secrets/cloudflare_tunnel_token
unset CLOUDFLARE_TUNNEL_TOKEN
chmod 600 secrets/cloudflare_tunnel_token
```

No agregues `secrets/` ni `jenkins_home/` a Git. El Compose monta el archivo en `/run/secrets/cloudflare_tunnel_token`; el token no aparece en la linea de comandos ni en las variables mostradas por `docker inspect`. Cloudflared se ejecuta como `root` dentro de su contenedor para leer el archivo con permisos `600`; ese contenedor no tiene montado el socket de Docker ni otros directorios del host.

## 6. Activar el Compose nuevo

Reemplaza el archivo Compose y levanta los servicios:

```bash
mv docker-compose.yml.new docker-compose.yml
docker compose up -d
docker compose ps
```

El nuevo Compose equivale a:

```yaml
services:
  jenkins:
    build:
      context: .
      dockerfile: Dockerfile.jenkins
    image: rokishi-jenkins:lts
    container_name: jenkins
    restart: unless-stopped
    user: root
    ports:
      - "8080:8080"
      - "50000:50000"
    volumes:
      - ./jenkins_home:/var/jenkins_home
      - /var/run/docker.sock:/var/run/docker.sock

  cloudflared:
    image: cloudflare/cloudflared:latest
    container_name: cloudflared
    restart: unless-stopped
    user: root
    depends_on:
      - jenkins
    environment:
      TUNNEL_TOKEN_FILE: /run/secrets/cloudflare_tunnel_token
    secrets:
      - cloudflare_tunnel_token
    command: tunnel --no-autoupdate run

secrets:
  cloudflare_tunnel_token:
    file: ./secrets/cloudflare_tunnel_token
```

En Cloudflare, el servicio del hostname de Jenkins debe apuntar a `http://jenkins:8080`, porque ambos servicios comparten la red creada por Compose.

## 7. Verificar Jenkins y Cloudflared

Confirma que Jenkins conserva los datos anteriores y que todas las herramientas existen:

```bash
docker exec jenkins java -version
docker exec jenkins git --version
docker exec jenkins go version
docker exec jenkins docker version
docker exec jenkins docker buildx version
docker exec jenkins heroku version
docker exec jenkins migrate -version
docker exec jenkins curl --version
```

Confirma el tunel sin mostrar sus variables de entorno:

```bash
docker logs --tail 50 cloudflared
```

Abre Jenkins mediante su hostname habitual y revisa que aparezcan los trabajos, plugins y credenciales anteriores. Si Jenkins aparece vacio, detente: el bind mount no apunta al `jenkins_home` original. Restaura `docker-compose.yml.before-heroku` antes de realizar otros cambios.

Cuando confirmes que todo funciona, edita `docker-compose.yml.before-heroku` y elimina el token antiguo que quedo dentro de ese respaldo. El token ya no sera valido despues de la rotacion, pero tampoco conviene conservarlo.

## 8. Crear la aplicacion y PostgreSQL en Heroku

Primero suscribe la cuenta personal al plan Eco desde Heroku Dashboard. Despues abre una terminal dentro de Jenkins:

```bash
docker exec -it jenkins bash
```

Dentro del contenedor:

```bash
heroku login
heroku create NOMBRE_DE_LA_APP --stack container
heroku addons:create heroku-postgresql:essential-0 \
  --app NOMBRE_DE_LA_APP \
  --wait
heroku pg:info --app NOMBRE_DE_LA_APP
heroku authorizations:create --description "Jenkins rokishi-back"
```

Copia el valor `Token` del ultimo comando. Heroku configura `DATABASE_URL`; no copies esa URL al repositorio ni la agregues manualmente a Jenkins.

## 9. Crear las credenciales de Jenkins

Abre **Manage Jenkins > Credentials > System > Global credentials** y crea dos credenciales **Secret text**:

| ID | Valor |
|---|---|
| `heroku-api-key` | Token generado por `authorizations:create` |
| `heroku-app-name` | Nombre exacto de la aplicacion Heroku |

El token nunca debe escribirse en `Jenkinsfile` ni en Git.

## 10. Crear el Pipeline

1. Crea un elemento **Multibranch Pipeline**.
2. Agrega GitHub como origen.
3. Selecciona el repositorio `rokishi-back`.
4. Agrega credenciales de GitHub si es privado.
5. Usa `Jenkinsfile` como ruta del script.
6. Ejecuta **Scan Multibranch Pipeline Now**.

El pipeline consulta GitHub cada cinco minutos sin necesitar un webhook entrante. En todas las ramas ejecuta pruebas y construye la imagen. Solo `main` aplica migraciones, publica la imagen en Heroku y comprueba `/api/health`.

La imagen se construye con:

```bash
docker buildx build \
  --platform linux/amd64 \
  --pull \
  --load \
  --tag rokishi-api:NUMERO_DE_BUILD \
  .
```

## 11. Primer despliegue

Fusiona los cambios en `main` y ejecuta el trabajo de esa rama. El pipeline debe completar estas etapas:

1. `Checkout`
2. `Test`
3. `Build image`
4. `Migrate database`
5. `Deploy`
6. `Verify`

Comprueba la API desde la Raspberry:

```bash
curl --fail --show-error \
  https://NOMBRE_DE_LA_APP.herokuapp.com/api/health
```

Respuesta esperada:

```json
{"status":"ok","database":"up"}
```

Consulta los registros cuando falle un despliegue:

```bash
docker exec jenkins heroku logs --tail --app NOMBRE_DE_LA_APP
```

## 12. Crear el primer administrador

El pipeline aplica la migracion `000006` antes de desplegar. Despues del primer despliegue consulta el codigo temporal que genera la API:

```bash
docker exec jenkins heroku logs --tail --app NOMBRE_DE_LA_APP
```

Busca el campo JSON `codigo_configuracion`, abre el frontend de Cloudflare Pages y completa el formulario inicial. No guardes ese codigo en Jenkins ni en GitHub: cambia cuando reinicia el proceso y deja de funcionar en cuanto se crea el primer usuario.

Confirma que `CORS_ALLOWED_ORIGINS` contiene el dominio exacto de Pages, sin ruta ni diagonal final:

```bash
docker exec jenkins heroku config:set \
  CORS_ALLOWED_ORIGINS=https://TU_PROYECTO.pages.dev \
  --app NOMBRE_DE_LA_APP
```

## 13. Operacion y espacio en disco

El pipeline elimina las etiquetas de imagen creadas durante cada build. Revisa el almacenamiento del Docker del host:

```bash
docker system df
```

Para eliminar cache de compilacion con mas de siete dias:

```bash
docker builder prune --filter until=168h --force
```

Este comando puede hacer mas lenta la siguiente compilacion y afecta a otros proyectos Docker de la Raspberry.

Revisa periodicamente el gasto de Heroku:

```bash
docker exec jenkins heroku addons --app NOMBRE_DE_LA_APP
docker exec jenkins heroku ps --app NOMBRE_DE_LA_APP
```

Debe existir una base Essential-0 y un solo proceso `web`.

## 14. Problemas comunes

- **Jenkins aparece vacio:** el Compose no esta montando `/home/charli/jenkins-lab/jenkins_home`.
- **`permission denied` en `docker.sock`:** confirma que Compose conserva `user: root` y monta `/var/run/docker.sock`.
- **`docker buildx` no existe:** reconstruye `rokishi-jenkins:lts` con `Dockerfile.jenkins`.
- **Heroku rechaza la arquitectura:** confirma `--platform linux/amd64`; Heroku Container Registry no acepta ARM64.
- **Cloudflared no conecta:** confirma que `secrets/cloudflare_tunnel_token` contiene el token rotado y revisa `docker logs cloudflared`.
- **El hostname muestra 502:** configura el servicio del tunel como `http://jenkins:8080`.
- **La migracion queda `dirty`:** revisa la migracion fallida antes de usar `force`.
- **Health devuelve 503:** la API arranco, pero PostgreSQL no esta disponible; revisa `heroku pg:info` y los logs.
- **La primera peticion tarda:** un dyno Eco estaba dormido y esta despertando.
- **El login responde bien pero vuelve a pedir acceso:** confirma que frontend y API usan HTTPS. Algunos bloqueadores rechazan cookies entre dominios distintos; la solucion estable es publicar ambos bajo subdominios del mismo dominio propio.

## Referencias

- [Jenkins en Docker](https://www.jenkins.io/doc/book/installing/docker/)
- [Tokens de Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/reference/tunnel-tokens/)
- [Parametros de Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/advanced/run-parameters/)
- [Heroku Container Registry](https://devcenter.heroku.com/articles/container-registry-and-runtime)
- [Provisionar Heroku Postgres](https://devcenter.heroku.com/articles/provisioning-heroku-postgres)
- [Autenticacion de Heroku CLI](https://devcenter.heroku.com/articles/authentication)
