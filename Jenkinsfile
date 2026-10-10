pipeline {
    agent any

    stages {
        stage('Build') {
            steps {
                bat 'go build ./... > build.log 2>&1 || (type build.log & exit /b 1)'
            }
        }

        stage('Test') {
            steps {
                bat 'go test ./... > test.log 2>&1 || (type test.log & exit /b 1)'
            }
        }
    }

    post {
        failure {
            echo '=== BUILD FAILED: capturing details ==='
            echo "Job: ${env.JOB_NAME}"
            echo "Build number: ${env.BUILD_NUMBER}"
            echo "Build URL: ${env.BUILD_URL}"
            bat 'if exist build.log type build.log'
            bat 'if exist test.log type test.log'
            archiveArtifacts artifacts: '*.log', allowEmptyArchive: true
        }
        success {
            echo 'Build succeeded.'
        }
    }
}