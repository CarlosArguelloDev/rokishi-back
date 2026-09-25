pipeline {
    agent any

    options {
        disableConcurrentBuilds()
        timestamps()
    }

    triggers {
        pollSCM('H/5 * * * *')
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
                      --pull \
                      --load \
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
                        set -eu
                        database_url="$(heroku config:get DATABASE_URL --app "$HEROKU_APP_NAME")"
                        test -n "$database_url"
                        "$HOME/go/bin/migrate" -path ./migrations -database "$database_url" up
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
                    string(credentialsId: 'heroku-api-key', variable: 'HEROKU_API_KEY'),
                    string(credentialsId: 'heroku-app-name', variable: 'HEROKU_APP_NAME')
                ]) {
                    sh '''
                        set -eu
                        registry_image="registry.heroku.com/${HEROKU_APP_NAME}/web"
                        trap 'docker image rm "$registry_image" >/dev/null 2>&1 || true' EXIT

                        printf '%s' "$HEROKU_API_KEY" | docker login --username=_ --password-stdin registry.heroku.com
                        docker tag "rokishi-api:${BUILD_NUMBER}" "$registry_image"
                        docker push "$registry_image"
                        heroku container:release web --app "$HEROKU_APP_NAME"
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
                    string(credentialsId: 'heroku-app-name', variable: 'HEROKU_APP_NAME')
                ]) {
                    retry(6) {
                        sleep time: 10, unit: 'SECONDS'
                        sh '''
                            set -eu
                            curl --fail --show-error --silent \
                              "https://${HEROKU_APP_NAME}.herokuapp.com/api/health"
                        '''
                    }
                }
            }
        }
    }

    post {
        always {
            sh 'docker logout registry.heroku.com >/dev/null 2>&1 || true'
            sh 'docker image rm "rokishi-api:${BUILD_NUMBER}" >/dev/null 2>&1 || true'
        }
    }
}
