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
                powershell 'go test ./...'
            }
        }

        stage('Build image') {
            steps {
                powershell '''
                    $ErrorActionPreference = 'Stop'
                    docker build --pull --tag "rokishi-api:$env:BUILD_NUMBER" .
                    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
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
                    powershell '''
                        $ErrorActionPreference = 'Stop'
                        $databaseUrl = heroku config:get DATABASE_URL --app $env:HEROKU_APP_NAME
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
                        if ([string]::IsNullOrWhiteSpace($databaseUrl)) { throw 'Heroku no devolvio DATABASE_URL' }

                        migrate -path .\migrations -database $databaseUrl up
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
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
                    powershell '''
                        $ErrorActionPreference = 'Stop'
                        $registryImage = "registry.heroku.com/$env:HEROKU_APP_NAME/web"

                        $env:HEROKU_API_KEY | docker login --username=_ --password-stdin registry.heroku.com
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

                        docker tag "rokishi-api:$env:BUILD_NUMBER" $registryImage
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

                        docker push $registryImage
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

                        heroku container:release web --app $env:HEROKU_APP_NAME
                        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
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
                        powershell '''
                            $ErrorActionPreference = 'Stop'
                            $health = Invoke-RestMethod "https://$env:HEROKU_APP_NAME.herokuapp.com/api/health"
                            if ($health.status -ne 'ok' -or $health.database -ne 'up') {
                                throw "Health check inesperado: $($health | ConvertTo-Json -Compress)"
                            }
                        '''
                    }
                }
            }
        }
    }

    post {
        always {
            powershell 'docker logout registry.heroku.com 2>$null; exit 0'
        }
    }
}
