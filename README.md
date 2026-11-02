# CI/CD Failure Explainer

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Jenkins](https://img.shields.io/badge/Jenkins-Pipeline_as_Code-D24939?logo=jenkins&logoColor=white)
![Slack](https://img.shields.io/badge/Slack-Notifications-4A154B?logo=slack&logoColor=white)

An AI-assisted DevOps tool that watches your Jenkins pipeline for build failures, sends the error log (with secrets removed) to an LLM, and delivers a plain-English explanation with a suggested fix to the Jenkins console and to Slack, so you spend less time digging through raw stack traces.

> **Status: work in progress.** The core pipeline (Jenkins hook, REST API, PostgreSQL storage, secret redaction, LLM analysis, Slack notifications) works end to end. A dashboard, full containerized deployment, and AWS deployment are next. See the [Roadmap](#roadmap).

<!-- Add screenshots once they are saved in docs/ (crop out any personal details):
![Jenkins console showing the AI explanation](docs/jenkins-console.png)
![Slack notification](docs/slack-notification.png) -->

---

## Why this exists

CI logs are long and cryptic. A failed build often means scrolling through hundreds of lines to find one useful error, and that error is not always obvious to a less experienced developer. This service turns the failure into a short explanation and a concrete next step, delivered where the developer is already looking: the Jenkins console and a Slack channel.

## How it works

```mermaid
flowchart LR
    A[Jenkins build fails] -->|log + source lines| B[Go service]
    B -->|1. redact secrets, save| C[(PostgreSQL)]
    B -->|2. redacted log| D[LLM API]
    D -->|3. explanation + fix| B
    B -->|4. save analysis| C
    B -->|5. notify| E[Slack]
    B -->|response| A
```

1. A Jenkins pipeline fails. The `post { failure { } }` block in the [`Jenkinsfile`](Jenkinsfile) collects the build log and, for Go compile errors, a few lines of source code around the failing line.
2. The hook sends the pipeline name, build number, build URL, log, and source context to the Go service (`POST /analyze-failure`).
3. The service validates the request, **redacts likely secrets**, and **saves the failure first**, so nothing is lost if a later step fails.
4. It sends the redacted log to an LLM, parses the reply into an explanation and a suggested fix, and stores both.
5. The result is posted to Slack in the background and returned to Jenkins, which prints it in the build console.

## Features

- **Jenkins integration** through a Pipeline-as-Code failure hook
- **REST API** with input validation, request-size limits, and proper HTTP status codes
- **Secret redaction** (cloud keys, tokens, passwords, private keys, URL credentials) before anything is stored, sent to the LLM, or posted to Slack
- **Source context**: for Go compile errors, Jenkins sends the lines around the failing line so the suggested fix is based on real code instead of a guess
- **PostgreSQL storage** of every failure with separate explanation and fix fields, and a queryable history (`GET /failures`)
- **Provider-agnostic LLM client** that works with any OpenAI-compatible API (Groq, Ollama, and others), switched by environment variables
- **Slack notifications** with the explanation, suggested fix, and a link back to the Jenkins build
- **Graceful degradation**: if the LLM is down or rate limited, the failure is still saved and Slack still gets the log tail
- **Fail-safe hook**: the explainer never breaks the build it monitors
- **Health endpoint** that also checks database connectivity
- **Unit tests** for redaction, reply parsing, and the Slack client
- **Vendored dependencies**, so Jenkins can build without network access

## Tech stack

| Area | Tools |
|---|---|
| Language | Go (standard library `net/http`, `database/sql`) |
| Database | PostgreSQL 16 (via the `pgx` driver) |
| CI | Jenkins (declarative pipeline) |
| Containers | Docker, Docker Compose |
| AI | Any OpenAI-compatible chat completions API |
| Notifications | Slack incoming webhooks |

## Project structure

```
.
├── cmd/server/main.go          # entry point: config, wiring, HTTP server
├── internal/
│   ├── api/handler.go          # HTTP handlers and validation
│   ├── db/db.go                # PostgreSQL access and migrations
│   ├── llm/
│   │   ├── client.go           # LLM client and prompt
│   │   └── parse.go            # splits the reply into explanation and fix
│   ├── redact/redact.go        # secret redaction
│   ├── slack/slack.go          # Slack webhook client
│   └── models/failure.go       # shared data types
├── vendor/                     # vendored dependencies
├── Jenkinsfile                 # build, test, and failure hook
├── docker-compose.yml          # PostgreSQL service
├── .env.example                # configuration template
└── README.md
```

Each package has `_test.go` files where tests exist (`redact`, `llm`, `slack`).

## Getting started

### Prerequisites

- Go 1.25 or newer (see `go.mod`)
- Docker Desktop (or Docker Engine) with Docker Compose
- Jenkins, if you want to try the pipeline hook
- A free API key from an OpenAI-compatible provider, for example [Groq](https://console.groq.com), or a local [Ollama](https://ollama.com) install (no key needed)
- Optional: a Slack workspace and an incoming webhook (see [Slack setup](#slack-setup))

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
Slack notifications enabled
Server starting on :8081
```

If `LLM_BASE_URL` or `LLM_MODEL` is not set, the service still runs and saves failures with AI analysis disabled. If `SLACK_WEBHOOK_URL` is not set, Slack notifications are disabled.

> Run only one copy of the service at a time. If port 8081 is already in use, stop the old process first.

### 4. Try it

```powershell
$body = @{
    pipeline_name  = "demo-pipeline"
    build_number   = "1"
    build_url      = "http://localhost:8080/job/demo-pipeline/1/"
    log_text       = "cmd\server\main.go:18:2: declared and not used: unused_var"
    source_context = "   17 | defer store.Close()`n>> 18 | unused_var := 42"
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:8081/analyze-failure -Method Post `
    -Body $body -ContentType "application/json" | Format-List
```

Or with curl:

```bash
curl -X POST http://localhost:8081/analyze-failure \
  -H "Content-Type: application/json" \
  -d '{"pipeline_name":"demo-pipeline","build_number":"1","log_text":"main.go:18:2: declared and not used: unused_var"}'
```

### 5. Run the tests

```powershell
go build ./...
go vet ./...
go test ./...
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
| `SLACK_WEBHOOK_URL` | Slack incoming webhook URL (optional; must start with `https://hooks.slack.com/`) | |

> **Notes**
> - Postgres reads `POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB` only when the data volume is first created. If you change them later, reset the volume with `docker compose down -v` (this deletes stored data).
> - Environment variables already set in your shell take priority over `.env`.
> - The Slack webhook URL and the API key are credentials. Keep them out of Git, screenshots, and chat.

**Using Ollama locally** (no key, nothing leaves your machine): set `LLM_BASE_URL=http://localhost:11434/v1`, leave `LLM_API_KEY` empty, and set `LLM_MODEL` to a model you have pulled.

### Slack setup

1. Go to [api.slack.com/apps](https://api.slack.com/apps), click **Create New App**, and choose **Blank app**.
2. Open **Incoming Webhooks** and turn **Activate Incoming Webhooks** on.
3. Click **Add New Webhook to Workspace**, choose a channel, and allow it.
4. Copy the webhook URL into `.env` as `SLACK_WEBHOOK_URL`.

## API reference

### `GET /health`

Returns `200 OK` when the service and database are reachable, `503` if the database is not.

### `POST /analyze-failure`

Request body:

```json
{
  "pipeline_name": "ci-failure-explainer",
  "build_number": "42",
  "build_url": "http://localhost:8080/job/ci-failure-explainer/42/",
  "log_text": "cmd/server/main.go:18:2: declared and not used: unused_var",
  "source_context": "   17 | defer store.Close()\n>> 18 | unused_var := 42"
}
```

`pipeline_name` and `log_text` are required. `build_number`, `build_url`, and `source_context` are optional. The request body is limited to 1 MB, `source_context` is capped at 4000 bytes, and `build_url` is only used if it is a plain `http(s)` link.

Response (`201 Created`):

```json
{
  "status": "received",
  "message": "failure saved and analyzed",
  "id": 7,
  "log_lines": 1,
  "explanation": "The build fails because the variable `unused_var` is declared but never used...",
  "fix": "Remove the declaration or use the variable..."
}
```

| Status | Meaning |
|---|---|
| `201` | Failure saved. `status` is `received` when the AI analysis succeeded, or `saved` when it was unavailable (the failure is still stored and Slack still gets a notification). |
| `400` | Invalid JSON or missing required fields |
| `405` | Wrong HTTP method (use `POST`) |
| `500` | The failure could not be saved |

### `GET /failures?limit=20`

Returns stored failures, newest first, each with `id`, `pipeline_name`, `build_number`, `log_excerpt`, `ai_explanation`, `fix_suggestion`, and `created_at`. `limit` is optional (default 20, maximum 100).

## Jenkins integration

The pipeline lives in the [`Jenkinsfile`](Jenkinsfile). It cleans stale logs, builds, tests, and on failure posts the logs to the service. This is a simplified excerpt of the failure hook (see the `Jenkinsfile` for the full version):

```groovy
post {
    failure {
        powershell '''
            $log = ""
            foreach ($f in "build.log", "test.log") {
                if (Test-Path $f) { $log += Get-Content $f -Raw }
            }

            # Grab a few source lines around the first "file.go:LINE:COL" in the log
            $ctx = ""
            # ... (reads 5 lines before and after the failing line) ...

            $body = @{
                pipeline_name  = $env:JOB_NAME
                build_number   = $env:BUILD_NUMBER
                build_url      = $env:BUILD_URL
                log_text       = $log
                source_context = $ctx
            } | ConvertTo-Json
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($body)
            try {
                $resp = Invoke-RestMethod -Uri "http://localhost:8081/analyze-failure" `
                    -Method Post -Body $bytes -ContentType "application/json" -TimeoutSec 60
                Write-Host $resp.explanation
                Write-Host $resp.fix
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
The build fails because the variable `unused_var` is declared but never
referenced in the function, which Go treats as a compile-time error.

=== SUGGESTED FIX ===
Remove the declaration or use the variable. For example, if the value is not
needed, delete the line; if you intended to use it, add a statement that
references it, such as `fmt.Println(unused_var)` or assign it to a field.
======================
```

The same explanation and fix are posted to the configured Slack channel, along with the pipeline name, build number, and a link to the Jenkins build.

## Design decisions

| Decision | Reasoning |
|---|---|
| Save the failure before calling the LLM | The AI step is a bonus layer and must never cause data loss. |
| Hook swallows errors (`try/catch`) | A monitoring add-on must not break the pipeline it monitors. |
| Redact secrets before storing, calling the LLM, or notifying Slack | Logs cross a trust boundary when sent to third parties. Redaction uses regex rules because it is deterministic, testable, and cannot be talked out of its job by injected text. |
| Provider-agnostic LLM client | Free tiers and model names change; switching providers is a config change, not a code change. |
| Plain `net/http` for the LLM call | No SDK dependency, so builds stay reproducible and the call is easy to reason about. |
| Log and source wrapped in tags and declared untrusted | Basic prompt-injection defense. Output is advisory only and is never executed. |
| Prompt forbids inventing code when no source is given | Reduces hallucinated fixes; real source lines are sent when available. |
| Parse the reply into two fields, with a fallback | Separate fields enable formatting and a dashboard; a badly formatted reply still ends up in the explanation instead of failing. |
| Slack posted from a background goroutine | Jenkins never waits on Slack, and a Slack outage cannot affect the API response. |
| Escape `& < >` in Slack messages | Slack treats `<...>` specially; model output is untrusted and could inject links or mentions. |
| Webhook URL stripped from network errors; prefix check on startup | The URL is a credential and must not reach logs, and the service only posts to Slack's own domain. |
| Path-traversal guard when Jenkins reads source files | The log is untrusted text, so a crafted path must not make the hook read files outside the workspace. |
| Parameterized SQL queries | Log text is untrusted input; placeholders prevent SQL injection. |
| Request-size limit and context timeouts | Protect the service from oversized payloads and slow dependencies. |
| Only the end of the log is stored | Build errors usually appear last, and it keeps rows and LLM input small. |
| Vendored dependencies | Jenkins builds without network access and results are reproducible. |
| Database bound to `127.0.0.1` | The database is not exposed to the network. |

## Security notes

- Secrets live in `.env` (gitignored) and are never passed through Jenkins. Only the Go service holds the LLM key and the Slack webhook URL.
- Likely secrets are redacted from logs and source context before storage, LLM calls, and Slack messages. Redaction is pattern-based and **best effort**: it reduces risk but is not a guarantee.
- **Build logs are still sent to the configured LLM provider.** Use a local model (Ollama) or an approved provider for real pipelines, and treat free hosted tiers as suitable for demo repositories only.
- Suggested fixes are advisory. Nothing is applied automatically; a human stays in the loop.

## Known limitations

- Source context is attached for Go compile errors (`file.go:LINE:COL`). Test failures and dependency errors are explained from the log alone, so their fixes are described in words.
- Redaction patterns cover common secret formats and can miss unusual ones.
- The LLM call is synchronous, so a slow provider delays the Jenkins hook (bounded by timeouts).
- Slack delivery is fire-and-forget with no retry queue.
- The Jenkins build link only works for people who can reach the Jenkins host (for example `localhost` on the same machine).
- Free-tier providers are rate limited. The service reports this and still saves the failure.
- Single-instance service with no authentication on the API; intended to run on a trusted network.

## Roadmap

- [x] Jenkins failure hook and REST API with validation
- [x] PostgreSQL storage with persistent volume and failure history
- [x] LLM analysis with graceful fallback
- [x] Parse the reply into separate explanation and fix fields
- [x] Prompt improvements, source context, and secret redaction
- [x] Slack notifications
- [x] Unit tests for redaction, parsing, and Slack
- [ ] Simple dashboard for failure history
- [ ] Containerize the service and run the full stack with Docker Compose
- [ ] Deploy to AWS EC2 with CI/CD
- [ ] Handler and database tests

## Author

**Mohd Shaiq**
[GitHub](https://github.com/ShaiqMohd) · [LinkedIn](https://www.linkedin.com/in/mohd-shaiq/)