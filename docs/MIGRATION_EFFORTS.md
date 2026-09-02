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
| Module and process lifecycle | ASP.NET Core `Program.cs` | `GoProject/cmd/jobdispatch/main.go` | `go test ./...`; shutdown wiring present | In progress |
| Domain model and state machine | `Models/`, `Services/JobService.cs` | `GoProject/internal/domain/`, `internal/service/` | Create/read unit tests | In progress |
| Persistence boundary | In-memory repository | `GoProject/internal/repository/` | Repository-backed service tests | In progress |
| HTTP API | Minimal API mappings in `Program.cs` | `GoProject/internal/httpapi/` | Create/read HTTP contract test | In progress |
| Background worker | `Services/JobWorkerService.cs` | `GoProject/internal/worker/` | Cancellation/retry tests | Not started |
| Health and operations | Health mappings and basic logging | `GoProject/internal/health/`, middleware | Health/shutdown/smoke checks | Not started |

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
- Known gap: claim, complete, fail, worker, and full configuration parity are the next slices.

**Learning notes**

`context.Context` is passed into repository and service calls so cancellation can be honored later. `sync.RWMutex` allows multiple readers while preventing concurrent map writes.

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
