# .NET to Go Migration Efforts

This document is the learning-oriented implementation log for the Job Dispatch port. It records what the .NET application does, how the Go version implements the same behavior, and why the code is different where Go has a more idiomatic approach.

The .NET application is the behavioral reference. A difference in syntax or framework is expected; a difference in externally observable behavior must be deliberate, tested, and recorded.

## Confirmed migration decisions

- Go HTTP routing uses the standard library `net/http` and `http.ServeMux`.
- Go observability uses basic `log/slog` logging.
- The Go toolchain will be pinned in `go.mod` using the current stable version available in the environment.

## Presentation rule

The primary audience is a short technical presentation, not a production-readiness review. Each migration entry should answer one question a listener can remember: what behavior had to stay the same, what changed in Go, and what evidence supports the result.

Prefer three deep comparisons over a catalog of every file:

1. How ASP.NET Core dependency injection maps to explicit Go construction.
2. How C# exceptions and framework responses map to returned Go errors and HTTP error mapping.
3. How `BackgroundService` and cancellation tokens map to a Go worker, `context.Context`, and graceful shutdown.

Database, metrics, deployment, and performance can be summarized as future work.

## How to use this document

For each migration slice, add one entry containing:

- **Reference behavior:** the user-visible and domain behavior in .NET.
- **.NET implementation:** source files and the relevant framework mechanism.
- **Go implementation:** source files and a plain-language explanation.
- **What is different:** explain the language/runtime difference and any trade-off.
- **How parity was checked:** tests, commands, or a manual example.
- **Learning notes:** define new Go concepts the first time they appear.

Use short examples and link to source files rather than copying large code blocks. Update this file in the same change as the implementation so the explanation remains aligned with the code.

## Migration status

| Slice | .NET reference | Go implementation | Parity evidence | Status |
|---|---|---|---|---|
| Contract and decisions | `.NetProject/CONTRACT_FREEZE.md` | `docs/MIGRATION_PLAN.md` | `dotnet test .NetProject/JobDispatch.slnx --no-restore --nologo` (9 passed) | Complete |
| Module and process lifecycle | ASP.NET Core `Program.cs` | `GoProject/cmd/jobdispatch/main.go` | `go test ./...`; `scripts/smoke-test.ps1` | Complete |
| Domain model and state machine | `Models/`, `Services/JobService.cs` | `GoProject/internal/domain/`, `internal/service/` | Parity tests for transitions, leases, retries, and idempotency | Complete |
| Persistence boundary | In-memory repository | `GoProject/internal/repository/` | Repository-backed service and eligibility tests | Complete |
| HTTP API | Minimal API mappings in `Program.cs` | `GoProject/internal/httpapi/` | Lifecycle, error, and health contract tests | Complete |
| Background worker | `Services/JobWorkerService.cs` | `GoProject/internal/worker/` | Completion, retry, exhaustion, and cancellation tests | Complete |
| Health and operations | Health mappings and basic logging | `GoProject/internal/httpapi/`, `cmd/jobdispatch` | Health contract tests and `scripts/smoke-test.ps1` | Complete |

## Go concepts used in this project

This section will grow as the port introduces concepts. Keep explanations practical and tied to this service.

- **Package:** a directory of related Go code. Names beginning with a lowercase letter are available only inside their package.
- **`struct`:** a data type containing named fields, used here for jobs and request data.
- **Interface:** a set of method signatures. The repository interface lets the service use storage without knowing its concrete database implementation.
- **`error`:** a returned value describing failure. Go normally checks errors explicitly instead of throwing exceptions for expected failures.
- **`context.Context`:** a request-scoped or process-scoped signal carrying cancellation and deadlines. The worker and database calls use it to stop promptly during shutdown.
- **Goroutine:** a lightweight concurrent function execution. The worker runs concurrently with the HTTP server.
- **Channel:** a typed communication mechanism between goroutines. Use one only when it clarifies coordination; a context and `sync.WaitGroup` may be sufficient for this MVP.

## 2026-09-02 — Module, create, and read slices

**Reference behavior**

`POST /jobs` creates a queued job with a generated identifier, timestamps, default attempts, and payload. `GET /jobs/{id}` returns the stored job or a stable not-found error.

**.NET implementation**

- `Program.cs` composes the application through ASP.NET Core dependency injection.
- `Services/JobService.cs` owns validation and job creation.
- `Services/InMemoryJobRepository.cs` stores jobs in a concurrent dictionary.

**Go implementation**

- `cmd/jobdispatch/main.go` explicitly constructs the repository, service, handler, and HTTP server.
- `internal/domain` contains plain structs and JSON tags.
- `internal/repository` protects the map with `sync.RWMutex`.
- `internal/service` returns ordinary Go `error` values.
- `internal/httpapi` maps service errors to HTTP responses.

**What is different and why**

Go has no framework-wide dependency-injection container in this implementation. Construction is visible in `main`, which makes dependencies easy to trace for a presentation and keeps the runtime behavior explicit.

**Parity evidence**

- Test: `Set-Location GoProject; go test ./...`
- Result: all create/read service and HTTP tests pass.
- This slice is covered by the state-machine and HTTP contract tests described below.

**Learning notes**

`context.Context` is passed into repository and service calls so cancellation can be honored later. `sync.RWMutex` allows multiple readers while preventing concurrent map writes.

## 2026-09-02 — State machine slice

**Reference behavior**

Claims add a lease and increment attempts. Live leases reject competing claims; expired leases can be claimed again. Completion is idempotent for completed jobs. Transient failures schedule a retry, while non-transient or exhausted failures become terminal.

**Go implementation**

- `internal/domain/job.go` defines lifecycle statuses and request types.
- `internal/service/job_service.go` implements transitions and sentinel errors.
- `internal/repository/memory.go` adds update and eligibility operations behind the repository interface.
- `internal/httpapi/handler.go` exposes claim, complete, and fail routes.

**What is different and why**

Expected failures are returned as Go `error` values and classified with `errors.Is`; there is no exception hierarchy. The HTTP layer translates those errors into the frozen status codes and error codes.

**Parity evidence**

- Test: `Set-Location GoProject; go test ./...`
- Result: all Go tests pass, including claim conflict, retry, completion idempotency, and HTTP lifecycle coverage.
- Known gap: worker integration was completed in the following slice.

## 2026-09-02 — Background worker slice

**Reference behavior**

The .NET hosted worker polls eligible jobs, processes them through the application service, continues after expected job failures, and exits when the host cancellation token is signaled.

**Go implementation**

- `internal/worker/worker.go` uses a goroutine-friendly `time.Ticker` loop.
- `context.Context` controls cancellation and causes the worker to stop promptly.
- `main.go` starts the worker explicitly and cancels it during process shutdown.
- `JobService.ProcessQueuedJob` applies the same deterministic normal, retry, and fail rules as the .NET reference.

**What is different and why**

Go does not provide an ASP.NET-style hosted-service abstraction here. The worker is a small explicit component whose `Run` method owns its ticker and select loop. Expected domain errors are logged and do not terminate the process.

**Parity evidence**

- Test: `Set-Location GoProject; go test ./...`
- Result: all Go tests pass, including worker completion, retry scheduling, and cancellation.
- Known gap: configuration and process smoke coverage were completed in the following slice.

**Learning notes**

`time.Ticker` sends periodic events on a channel. A `select` waits for either the next tick or `ctx.Done()`, which is the idiomatic Go pattern for cancelable background work.

## 2026-09-02 — Configuration, health, and contract parity slice

**Reference behavior**

The process accepts environment-based configuration, exposes timestamped health responses, and supports the complete create, claim, complete, and fail HTTP lifecycle with stable error codes.

**Go implementation**

- `internal/config/config.go` loads defaults and `JobDispatch__...` environment variables.
- `internal/config/config_test.go` verifies parsing and validation.
- `internal/httpapi/handler.go` includes UTC timestamps on health responses.
- `internal/httpapi/handler_test.go` covers lifecycle, validation, not-found, failure idempotency, and health contracts.
- `scripts/smoke-test.ps1` builds and runs the real process, checks readiness, executes a lifecycle flow, and stops the child process.

**What is different and why**

Go configuration is parsed explicitly instead of using the ASP.NET configuration provider pipeline. The smoke script makes process lifecycle behavior reproducible on Windows and demonstrates targeted child-process cleanup.

**Parity evidence**

- Tests: `Set-Location GoProject; go test ./...`
- Smoke test: `Set-Location GoProject; .\scripts\smoke-test.ps1`
- Result: tests and the real-process lifecycle flow pass.
- Known gap: the presentation-critical in-memory port is complete; database, metrics, tracing, containers, and other production features remain deferred.

## 2026-09-02 — Parity hardening

**Reference behavior**

The Go implementation now has explicit evidence for the edge cases frozen by the .NET reference: retry timing, expired lease recovery, terminal failure, idempotency, and worker exhaustion.

**Go implementation**

- `internal/service/job_service_test.go` covers not-due retries, expired leases, non-transient terminal failure, and repeated terminal operations.
- `internal/worker/worker_test.go` covers a `fail` job reaching terminal failure after attempts are exhausted.
- `internal/httpapi/handler_test.go` covers the externally visible HTTP error and health contracts.

**What is different and why**

The behavior is intentionally equivalent. The tests use Go's direct function calls and `httptest` instead of ASP.NET's `WebApplicationFactory`, while preserving the same observable outcomes.

**Parity evidence**

- Test: `Set-Location GoProject; go test ./...`
- Result: all Go packages pass.
- Known gap: production persistence and distributed concurrency remain outside the presentation scope.

**Learning notes**

Parity hardening means testing edge cases against the .NET contract so implementation differences are deliberate rather than accidental.

## Entry template

### [Date] — [Migration slice]

**Reference behavior**

Describe what a caller or worker observes.

**.NET implementation**

- Files:
- Framework/runtime concepts:

**Go implementation**

- Files:
- Plain-language explanation:

**What is different and why**

Describe the meaningful difference without claiming that different syntax is a behavior difference.

**Parity evidence**

- Test or command:
- Result:
- Known gap, if any:

**Learning notes**

Explain any new Go concept needed to understand this slice.

## CONSIDERATIONS BEYOND PROJECT SCOPE: Intentional Differences and Future Work

Record approved differences here with a reason and the evidence that the user-visible contract remains equivalent. Do not use this section to hide unfinished work; unfinished work belongs in the migration status table and the plan's gap list.

Potential future slices include PostgreSQL persistence, SQL migrations, database-backed readiness, Prometheus metrics, distributed tracing, Docker packaging, and image comparisons. These are not required to complete the short presentation.
