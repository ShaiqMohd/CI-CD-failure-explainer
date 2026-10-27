pipeline {
    agent any

    stages {
        stage('Prepare') {
            steps {
                bat 'if exist build.log del /q build.log'
                bat 'if exist test.log del /q test.log'
            }
        }

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
            echo '=== BUILD FAILED: sending log to explainer service ==='
            powershell '''
                [Console]::OutputEncoding = [System.Text.Encoding]::UTF8

                $log = ""
                foreach ($f in "build.log", "test.log") {
                    if (Test-Path $f) { $log += Get-Content $f -Raw }
                }

                # Find the first "file.go:LINE:COL" in the log and grab nearby source lines
                $ctx = ""
                if ($log -match "([^ :]+[.]go):([0-9]+):[0-9]+") {
                    $file = $Matches[1]
                    $line = [int]$Matches[2]
                    if ((Test-Path $file) -and ($file -notmatch "[.][.]") -and ($file -notmatch "^[A-Za-z]:")) {
                        $all = Get-Content $file
                        $start = [Math]::Max(1, $line - 5)
                        $end = [Math]::Min($all.Count, $line + 5)
                        for ($i = $start; $i -le $end; $i++) {
                            $mark = "  "
                            if ($i -eq $line) { $mark = ">>" }
                            $ctx += "$mark $i | $($all[$i-1])`n"
                        }
                    }
                }

                $body = @{
                    pipeline_name  = $env:JOB_NAME
                    build_number   = $env:BUILD_NUMBER
                    log_text       = $log
                    source_context = $ctx
                } | ConvertTo-Json

                $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)

                try {
                    $resp = Invoke-RestMethod -Uri "http://localhost:8081/analyze-failure" `
                        -Method Post -Body $bytes -ContentType "application/json" -TimeoutSec 60
                    Write-Host "Explainer responded: $($resp.status) - $($resp.message)"
                    if ($resp.explanation) {
                        Write-Host ""
                        Write-Host "=== AI EXPLANATION ==="
                        Write-Host $resp.explanation
                        if ($resp.fix) {
                            Write-Host ""
                            Write-Host "=== SUGGESTED FIX ==="
                            Write-Host $resp.fix
                        }
                        Write-Host "======================"
                    }
                } catch {
                    Write-Host "Could not reach explainer service: $_"
                }
            '''
            archiveArtifacts artifacts: '*.log', allowEmptyArchive: true
        }
        success {
            echo 'Build succeeded.'
        }
    }
}