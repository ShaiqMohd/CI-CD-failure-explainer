# CI/CD Failure Explainer

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Jenkins](https://img.shields.io/badge/Jenkins-Pipeline_as_Code-D24939?logo=jenkins&logoColor=white)

An AI-assisted DevOps tool that watches your Jenkins pipeline for build failures, sends the error log to an LLM, and returns a plain-English explanation with a suggested fix, so you spend less time digging through raw stack traces.

> **Status: work in progress.** The core pipeline (Jenkins hook, REST API, PostgreSQL storage, LLM analysis) works end to end. Slack delivery, a dashboard, and full containerized deployment are in progress. See the [Roadmap](#roadmap).

<!-- Add a screenshot of the Jenkins console showing the AI explanation here:
![Jenkins console showing the AI explanation](docs/jenkins-console.png) -->

---

## Why this exists

CI logs are long and cryptic. A failed build often means scrolling through hundreds of lines to find one useful error, and that error is not always obvious to a less experienced developer. This service turns the failure into a short explanation and a concrete next step, delivered where the developer is already looking (the Jenkins console today, Slack next).

## How it works

```mermaid
flowchart LR
    A[Jenkins build fails] -->|post failure hook| B[Go service]
    B -->|1. save log| C[(PostgreSQL)]
    B -->|2. send log| D[LLM API]
    D -->|3. explanation| B
    B -->|4. save analysis| C
    B -->|response| A
```

1. A Jenkins pipeline fails. The `post { failure { } }` block in the [`Jenkinsfile`](Jenkinsfile) collects the build log.
2. The hook sends the pipeline name, build number, and log to the Go service (`POST /analyze-failure`).
3. The service validates the request and **saves the failure first**, so nothing is lost if the AI step fails.
4. It sends the log to an LLM and stores the explanation next to the failure.
5. The explanation is returned to Jenkins and printed in the build console.

## Features

- **Jenkins integration** through a Pipeline-as-Code failure hook
- **REST API** with input validation, request-size limits, and proper HTTP status codes
- **PostgreSQL storage** of every failure and its analysis, with a queryable history (`GET /failures`)
- **Provider-agnostic LLM client** that works with any OpenAI-compatible API (Groq, Ollama, and others), switched by environment variables
- **Graceful degradation**: if the LLM is down or rate limited, the failure is still saved and the build is unaffected
- **Fail-safe hook**: the explainer never breaks the build it monitors
- **Health endpoint** that also checks database connectivity
- **Vendored dependencies**, so Jenkins can build without network access

## Tech stack

| Area | Tools |
|---|---|
| Language | Go (standard library `net/http`, `database/sql`) |
| Database | PostgreSQL 16 (via the `pgx` driver) |
| CI | Jenkins (declarative pipeline) |
| Containers | Docker, Docker Compose |
| AI | Any OpenAI-compatible chat completions API |

## Project structure

```
.
├── cmd/server/main.go        # entry point: config, wiring, HTTP server
├── internal/
│   ├── api/handler.go        # HTTP handlers and validation
│   ├── db/db.go              # PostgreSQL access and migrations
│   ├── llm/client.go         # LLM client and prompt
│   └── models/failure.go     # shared data types
├── vendor/                   # vendored dependencies
├── Jenkinsfile               # build, test, and failure hook
├── docker-compose.yml        # PostgreSQL service
├── .env.example              # configuration template
└── README.md
```

## Getting started

### Prerequisites

- Go 1.25 or newer (see `go.mod`)
- Docker Desktop (or Docker Engine) with Docker Compose
- Jenkins, if you want to try the pipeline hook
- A free API key from an OpenAI-compatible provider, for example [Groq](https://console.groq.com), or a local [Ollama](https://ollama.com) install (no key needed)

### 1. Clone and configure

```powershell
git clone https://github.com/ShaiqMohd/CI-CD-failure-explainer.git
cd CI-CD-failure-explainer
Copy-Item .env.example .env      # on Linux/macOS: cp .env.example .env
```

Edit `.env` and fill in your values. **Never commit `.env`**: it is listed in `.gitignore`.

### 2. Start PostgreSQL

```powershell
docker compose up -d
docker compose ps        # wait until the status is "healthy"
```

Data is stored in a named Docker volume, so it survives container restarts. Postgres listens on `127.0.0.1:5433` so it does not clash with a local install and is not exposed to your network.

### 3. Run the service

```powershell
go run ./cmd/server
```

You should see:

```
LLM analysis enabled (model: ...)
Server starting on :8081
```

If `LLM_BASE_URL` or `LLM_MODEL` is not set, the service still runs and saves failures, with AI analysis disabled.

### 4. Try it

```powershell
$body = @{
    pipeline_name = "demo-pipeline"
    build_number  = "1"
    log_text      = "cmd\server\main.go:17:2: declared and not used: unusedVar"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/analyze-failure -Method Post `
    -Body $body -ContentType "application/json" | Format-List
```

Or with curl:

```bash
curl -X POST http://localhost:8081/analyze-failure \
  -H "Content-Type: application/json" \
  -d '{"pipeline_name":"demo-pipeline","build_number":"1","log_text":"main.go:17:2: declared and not used: unusedVar"}'
```

## Configuration

All settings come from environment variables (loaded from `.env` for local runs).

| Variable | Description | Example |
|---|---|---|
| `POSTGRES_USER` | Database user, used by Docker Compose | `failuser` |
| `POSTGRES_PASSWORD` | Database password (letters and numbers avoid URL-escaping problems) | `change_me` |
| `POSTGRES_DB` | Database name | `failuresdb` |
| `DATABASE_URL` | Connection string used by the service | `postgres://failuser:change_me@localhost:5433/failuresdb?sslmode=disable` |
| `LLM_BASE_URL` | Base URL of an OpenAI-compatible API | `https://api.groq.com/openai/v1` |
| `LLM_API_KEY` | API key (leave empty for local providers like Ollama) | |
| `LLM_MODEL` | Chat model ID from your provider | |

> **Note:** Postgres reads `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB` only when the data volume is first created. If you change them later, reset the volume with `docker compose down -v` (this deletes stored data).

**Using Ollama locally** (no key, nothing leaves your machine): set `LLM_BASE_URL=http://localhost:11434/v1`, leave `LLM_API_KEY` empty, and set `LLM_MODEL` to a model you have pulled.

## API reference

### `GET /health`

Returns `200 OK` when the service and database are reachable, `503` if the database is not.

### `POST /analyze-failure`

Request body:

```json
{
  "pipeline_name": "ci-failure-explainer",
  "build_number": "42",
  "log_text": "cmd/server/main.go:18:2: declared and not used: unused_var"
}
```

`pipeline_name` and `log_text` are required. The request body is limited to 1 MB.

Response (`201 Created`):

```json
{
  "status": "received",
  "message": "failure saved and analyzed",
  "id": 7,
  "log_lines": 2,
  "explanation": "Explanation: ... Suggested Fix: ..."
}
```

| Status | Meaning |
|---|---|
| `201` | Failure saved. `status` is `received` when the AI analysis succeeded, or `saved` when it was unavailable (the failure is still stored). |
| `400` | Invalid JSON or missing required fields |
| `405` | Wrong HTTP method (use `POST`) |
| `500` | The failure could not be saved |

### `GET /failures?limit=20`

Returns stored failures, newest first. `limit` is optional (default 20, maximum 100).

## Jenkins integration

The pipeline lives in the [`Jenkinsfile`](Jenkinsfile). It cleans stale logs, builds, tests, and on failure posts the logs to the service. The key part is the failure hook:

```groovy
post {
    failure {
        powershell '''
            $log = ""
            foreach ($f in "build.log", "test.log") {
                if (Test-Path $f) { $log += Get-Content $f -Raw }
            }
            $body = @{
                pipeline_name = $env:JOB_NAME
                build_number  = $env:BUILD_NUMBER
                log_text      = $log
            } | ConvertTo-Json
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
            try {
                $resp = Invoke-RestMethod -Uri "http://localhost:8081/analyze-failure" `
                    -Method Post -Body $bytes -ContentType "application/json" -TimeoutSec 60
                Write-Host $resp.explanation
            } catch {
                Write-Host "Could not reach explainer service: $_"
            }
        '''
    }
}
```

**Setup:** create a Jenkins **Pipeline** job, choose **Pipeline script from SCM**, point it at this repository, and set the script path to `Jenkinsfile`. The example uses `bat` and `powershell` steps because it was built on a Windows Jenkins agent. On a Linux agent, replace them with `sh` steps.

### Example output

A deliberately broken build produces this in the Jenkins console:

```
cmd\server\main.go:18:2: declared and not used: unused_var
...
Explainer responded: received - failure saved and analyzed

=== AI EXPLANATION ===
Explanation: The Go compiler reports that a variable named `unused_var` was
declared in `cmd/server/main.go` but never referenced, causing a build failure.
Suggested Fix: Remove the unused declaration or use the variable, then rebuild.
======================
```

## Design decisions

| Decision | Reasoning |
|---|---|
| Save the failure before calling the LLM | The AI step is a bonus layer and must never cause data loss. |
| Hook swallows errors (`try/catch`) | A monitoring add-on must not break the pipeline it monitors. |
| Provider-agnostic LLM client | Free tiers and model names change; switching providers is a config change, not a code change. |
| Plain `net/http` for the LLM call | No SDK dependency, so builds stay reproducible and the call is easy to reason about. |
| Log wrapped in `<log>` tags and declared untrusted | Basic prompt-injection defense. Output is advisory only and is never executed. |
| Parameterized SQL queries | Log text is untrusted input; placeholders prevent SQL injection. |
| Request-size limit and context timeouts | Protect the service from oversized payloads and slow dependencies. |
| Only the end of the log is stored | Build errors usually appear last, and it keeps rows and LLM input small. |
| Vendored dependencies | Jenkins builds without network access and results are reproducible. |
| Database bound to `127.0.0.1` | The database is not exposed to the network. |

## Security notes

- Secrets live in `.env` (gitignored) and are never passed through Jenkins. Only the Go service holds the API key.
- **Build logs are sent to the configured LLM provider.** Logs can contain secrets or internal paths, so use a local model (Ollama) or an approved provider for real pipelines. Automatic secret redaction is on the roadmap.
- Suggested fixes are advisory. Nothing is applied automatically; a human stays in the loop.

## Known limitations

- The model sees only the log, so example code in its suggestions can be a guess and may not match your source. Sending surrounding source lines is planned.
- The LLM call is synchronous, so a slow provider delays the Jenkins hook (bounded by timeouts).
- Free-tier providers are rate limited. The service reports this and still saves the failure.
- Single-instance service with no authentication on the API; intended to run on a trusted network.

## Roadmap

- [x] Jenkins failure hook and REST API with validation
- [x] PostgreSQL storage with persistent volume and failure history
- [x] LLM analysis with graceful fallback
- [ ] Parse the reply into separate explanation and fix fields
- [ ] Prompt improvements and secret redaction before logs leave the machine
- [ ] Slack notifications
- [ ] Simple dashboard for failure history
- [ ] Containerize the service and run the full stack with Docker Compose
- [ ] Deploy to AWS EC2 with CI/CD
- [ ] Automated tests

## Author

**Mohd Shaiq**
[GitHub](https://github.com/ShaiqMohd) · [LinkedIn](https://www.linkedin.com/in/mohd-shaiq/)
