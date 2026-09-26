pipeline {
    agent any

    options {
        disableConcurrentBuilds()
        timestamps()
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Test') {
            steps {
                sh 'go test ./...'
            }
        }

        stage('Build image') {
            steps {
                sh '''
                    set -eu

                    docker buildx build \
                    --platform linux/amd64 \
                    --provenance=false \
                    --sbom=false \
                    --output=type=docker,oci-mediatypes=false \
                    --pull \
                    --tag "rokishi-api:${BUILD_NUMBER}" \
                    .
                '''
            }
        }

        stage('Migrate database') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    string(credentialsId: 'heroku-api-key', variable: 'HEROKU_API_KEY'),
                    string(credentialsId: 'heroku-app-name', variable: 'HEROKU_APP_NAME')
                ]) {
                    sh '''
                        set +x
                        set -eu
                        database_url="$(heroku config:get DATABASE_URL --app "$HEROKU_APP_NAME")"
                        test -n "$database_url"
                        migrate -path ./migrations -database "$database_url" up
                        unset database_url
                    '''
                }
            }
        }

        stage('Deploy') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    string(
                        credentialsId: 'heroku-api-key',
                        variable: 'HEROKU_API_KEY'
                    ),
                    string(
                        credentialsId: 'heroku-app-name',
                        variable: 'HEROKU_APP_NAME'
                    )
                ]) {
                    sh '''
                        set +x
                        set -eu

                        export DOCKER_CONFIG
                        DOCKER_CONFIG="$(mktemp -d)"

                        registry_image="registry.heroku.com/${HEROKU_APP_NAME}/web"

                        trap 'rm -rf "$DOCKER_CONFIG"' EXIT

                        printf '%s' "$HEROKU_API_KEY" |
                            docker login \
                            --username=_ \
                            --password-stdin \
                            registry.heroku.com

                        docker tag \
                        "rokishi-api:${BUILD_NUMBER}" \
                        "$registry_image"

                        docker push "$registry_image"

                        heroku container:release web \
                        --app "$HEROKU_APP_NAME"
                    '''
                }
            }
        }

        stage('Verify') {
            when {
                branch 'main'
            }
            steps {
                withCredentials([
                    string(
                        credentialsId: 'heroku-app-url',
                        variable: 'HEROKU_APP_URL'
                    )
                ]) {
                    retry(6) {
                        sleep time: 10, unit: 'SECONDS'

                        sh '''
                            set -eu

                            curl --fail \
                                --show-error \
                                --silent \
                                "${HEROKU_APP_URL}/api/health"
                        '''
                    }
                }
            }
        }
    }

    post {
        always {
            sh 'docker image rm "rokishi-api:${BUILD_NUMBER}" >/dev/null 2>&1 || true'
        }
    }
}
