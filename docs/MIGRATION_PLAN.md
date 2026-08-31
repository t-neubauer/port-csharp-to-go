# .NET to Go Migration Plan

## Decision Summary

Build the .NET application first as the prerequisite for the migration. The baseline application will be a small **Job Dispatch API**: clients submit jobs, workers claim them, and a hosted background processor retries transient failures and records the final outcome.

This is intentionally more representative than a CRUD sample. It creates a stable behavioral contract that can be ported to Go and compared across runtime behavior, architecture, operational concerns, and developer experience.

**Timebox:** six working days, approximately three focused hours per day, with one developer supported by the Prometheus, Archie, and Devon agents. Pin the runtime, database, router, migration tool, and container base-image versions on Day 1.

## Objectives

- Deliver a runnable, tested .NET baseline before porting.
- Port the same externally observable behavior to Go without redesigning the domain.
- Compare equivalent production concerns, not just syntax and HTTP routing.
- Produce evidence for a technical presentation: tests, timings, container behavior, and documented trade-offs.

## Baseline Application

### Domain

A job represents work requested by a client.

- `POST /jobs` creates a queued job.
- `GET /jobs/{id}` returns the job and its status.
- `POST /jobs/{id}/claim` claims a queued job for processing.
- `POST /jobs/{id}/complete` marks a claimed job complete.
- `POST /jobs/{id}/fail` records a failure and either schedules a retry or marks the job permanently failed.
- A hosted worker periodically claims eligible jobs and simulates processing.

The simulated handler should be deterministic and configurable so tests can exercise success, transient failure, retry exhaustion, and cancellation without an external third-party system. The hosted worker calls the same application service used by the HTTP handlers; transition logic must not be duplicated in the worker.

### Why this is the right baseline

The application is small enough to finish in six days, but it makes the migration confront real boundaries:

- HTTP contracts, validation, status codes, and error payloads.
- Dependency composition and lifecycle management.
- Environment-based configuration and startup validation.
- SQL persistence, transactions, schema migration, and optimistic/concurrent claiming.
- Background workers, cancellation, retry backoff, and idempotency.
- Structured logs, request correlation, and basic metrics or traces.
- Liveness/readiness health checks and graceful shutdown.
- Container builds, non-root execution, and reproducible local startup.

### Contract decisions

- Define exact request and response fields, UTC timestamp rules, status codes, and the error payload on Day 1.
- Use one relational database and one migration tool in both implementations. The schema includes `attempt_count`, `max_attempts`, `next_attempt_at`, `lease_owner`, `lease_expires_at`, and terminal timestamps.
- Claiming is a transaction that takes an expiring lease. A worker crash makes an expired lease claimable again; work is at-least-once, not exactly-once.
- Retry uses a fixed, documented bounded backoff. Attempts increment when a claim is made, and exhausted jobs become terminally failed.
- Readiness means the database is reachable and migrations have completed. Liveness has no database dependency.
- The MVP supports one service instance and one worker loop. Distributed coordination is explicitly deferred.
- The worker stops polling on shutdown, rejects new claims, and waits a bounded timeout for in-flight work before exiting.

### Deliberate exclusions

These are outside the six-day MVP unless the core path finishes early:

- User accounts, OAuth, and multi-tenant authorization.
- A real external queue or cloud provider.
- A web frontend.
- Kubernetes production deployment.
- High-scale performance tuning or distributed tracing infrastructure.
- Exactly-once delivery claims. The design should document at-least-once processing and make handlers idempotent.

## MVP Acceptance Criteria

The MVP is complete only when all of the following are true for both implementations, unless explicitly marked as a known parity gap:

- A clean checkout starts the service and database with documented commands.
- The API exposes the five job operations above plus `/health/live` and `/health/ready`.
- Invalid requests return consistent, documented `4xx` responses; unknown jobs return `404`.
- Job transitions enforce valid state changes and cannot claim the same queued job concurrently.
- A transient failure retries with bounded attempts and deterministic backoff; exhaustion produces a terminal failed state.
- Repeating a completion or failure request is idempotent and does not corrupt state.
- Configuration is supplied through environment variables with documented defaults and startup validation for required values.
- Logs are structured and include request correlation plus job identifiers where applicable.
- Metrics include named counters for created, claimed, completed, retried, and failed jobs; distributed tracing is deferred.
- Graceful shutdown stops new work and allows in-flight work to finish within a bounded timeout.
- Unit tests cover state transitions, retry policy, validation, and idempotency.
- Integration tests cover the HTTP contract, persistence, health endpoints, and at least one concurrent-claim case.
- Both services build into small runnable containers and run as a non-root user.
- A parity checklist records endpoint behavior, persistence behavior, worker behavior, and known differences.

A feature is not MVP-complete merely because it compiles. The acceptance evidence must include test results and a short manual smoke test for each implementation.

## Requirements Matrix

| Area | MVP requirement | Evidence |
|---|---|---|
| API | Create, read, claim, complete, and fail jobs | Contract tests and curl/http examples |
| Domain | Explicit state machine with legal transitions | Unit tests |
| Storage | Relational schema, migrations, repository boundary, transaction for claiming | Integration tests |
| Worker | Poll, claim, process, retry, stop on cancellation | Worker tests and logs |
| Reliability | Idempotent terminal operations and bounded retries | Repeated-request and retry tests |
| Operations | Configuration, structured logs, correlation, counters, liveness/readiness | Configuration test and captured output |
| Delivery | Docker build and local compose startup | Build/run command and smoke test |
| Migration | Equivalent Go behavior and documented divergences | Parity checklist |

## Six-Day Execution Plan

### Day 1: Shape the contract and skeleton

**Goal:** remove ambiguity before implementation.

- Lock the versions, repository layout, database choice, migration tool, schema, and local run commands.
- Define the job state machine, request/response schemas, error format, retry rules, and configuration keys.
- Define the claim transaction, lease expiry behavior, UTC timestamp rules, and who runs migrations.
- Create the .NET solution, projects, dependency composition, health endpoints, and initial container setup.
- Add a minimal vertical slice: create and read a job.

**Checkpoint:** the .NET service starts locally, its contract is written down, and create/read work end to end.

### Day 2: Complete the .NET domain path

**Goal:** finish the behavior that Go must reproduce.

- Implement claim, complete, and fail transitions.
- Add the relational schema and migrations.
- Make claiming transactional and safe under concurrent requests.
- Add validation, consistent errors, idempotency rules, and repository tests.

**Checkpoint:** .NET API and persistence tests pass; the state machine is stable enough to port.

### Day 3: Make the .NET baseline production-shaped

**Goal:** finish operational behavior and capture the reference evidence.

- Add the hosted worker, cancellation, bounded retry/backoff, and graceful shutdown.
- Add structured logging, correlation IDs, and counters for created, claimed, completed, retried, and failed jobs.
- Add liveness/readiness checks, startup configuration validation, integration tests, and a non-root Docker image.
- Record baseline build/run time, image size, test count, and a smoke-test transcript.

**Checkpoint:** the .NET implementation meets the MVP criteria or has a short, explicit gap list. Freeze its API and behavior before porting.

### Day 4: Port the core to Go

**Goal:** reproduce the contract and domain behavior first.

- Create the Go module and map the .NET boundaries to idiomatic Go packages.
- Implement configuration, HTTP handlers, validation, domain state transitions, repository, migrations, and health endpoints.
- Port the unit and integration tests alongside the implementation.

**Checkpoint:** Go create/read/claim/complete/fail flows pass the same contract cases as .NET.

### Day 5: Port operations and close parity gaps

**Goal:** make Go runnable and operationally comparable.

- Port the worker, cancellation, retry policy, idempotency, structured logs, correlation, and readiness behavior.
- Build and run the Go container with the same database and environment contract.
- Execute the shared smoke test and concurrency test against both services.
- Resolve only parity defects and record intentional differences with reasons.

**Checkpoint:** both implementations pass the shared acceptance suite and can be started from documented commands.

### Day 6: Compare, document, and present

**Goal:** turn implementation work into defensible project results.

- Run a small repeatable comparison: build duration, test result, container image size, startup behavior, and one representative smoke scenario.
- Review code and document .NET-to-Go mappings, trade-offs, risks, and remaining production work.
- Prepare the presentation and demo script: baseline problem, architecture, porting decisions, evidence, parity gaps, and lessons learned.
- Reserve the final hour for fixing only release-blocking issues.

**Exit checkpoint:** reproducible demo, passing MVP checks, comparison notes, and a clear list of known limitations.

## Architecture and Porting Map

| Concern | .NET baseline | Go target | Comparison focus |
|---|---|---|---|
| HTTP | ASP.NET Core endpoints/controllers | `net/http` with a small router | Binding, middleware, status/error behavior |
| Composition | Built-in dependency injection and hosted service | Explicit constructors and lifecycle-managed worker | Implicit versus explicit dependency wiring |
| Domain | C# records/enums and service layer | Go structs, constants, and service package | Modeling, error flow, and test ergonomics |
| Persistence | SQL repository with migrations | Equivalent SQL repository and migrations | Transactions, scanning, and resource ownership |
| Background work | `BackgroundService` and cancellation tokens | Goroutine plus context cancellation | Shutdown semantics and concurrency clarity |
| Configuration | Options binding and startup validation | Environment parser with explicit validation | Defaults, failure timing, and discoverability |
| Observability | Structured logging plus correlation middleware | Structured logger plus request middleware | Context propagation and useful fields |
| Health | ASP.NET Core health checks | Explicit liveness/readiness handlers | Dependency checks and orchestration semantics |
| Delivery | Multi-stage non-root Dockerfile | Multi-stage non-root Dockerfile | Image size, startup, and operational parity |

The port should preserve the API contract and domain semantics. It should not mechanically imitate .NET abstractions where Go's explicit error handling, constructors, interfaces, and context model provide a clearer equivalent.

## Ownership and Working Agreements

- **Prometheus:** maintain this plan, requirements, checkpoints, risks, and final narrative.
- **Archie:** review the baseline architecture, production concerns, trade-offs, and parity claims.
- **Devon:** implement the .NET baseline, Go port, Dockerfiles, tests, and comparison evidence.
- **User:** make scope decisions at each checkpoint and approve any MVP gap that cannot fit the timebox.

The working rule is to freeze behavior after Day 3. New features after that point require removing an equal amount of scope.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| The worker expands into a queue platform | Keep processing in-process and deterministic; use an interface only at the handler boundary. |
| Claimed work is stranded after a worker crash | Use expiring leases and test that an expired claim can be recovered. |
| Database setup consumes the schedule | Pin one local containerized relational database and provide one compose command. |
| “Production-level” becomes too broad | Treat health, shutdown, logs, config, retries, tests, and containers as the fixed operational slice; defer auth and Kubernetes. |
| .NET baseline is not stable before porting | Freeze the contract at the Day 3 checkpoint; port tests with the behavior. |
| Language comparisons become subjective | Capture the same commands, scenarios, and metrics for both implementations. |
| Concurrency behavior differs subtly | Use a shared concurrent-claim test and inspect database effects, not only HTTP responses. |

## Stretch Goals

Only start these after every MVP criterion passes:

- OpenTelemetry exporter or a richer trace demonstration.
- A real queue adapter behind the processing boundary.
- Authentication middleware with one protected endpoint.
- Kubernetes manifests for deployment, probes, and configuration.
- A load-test scenario with a short interpretation of results.

## Presentation Outline

1. Problem and scope: why this service was chosen for a meaningful port.
2. .NET baseline: architecture, domain flow, and operational surface.
3. Porting map: what stayed equivalent and what became idiomatic Go.
4. Production challenges: persistence concurrency, worker lifecycle, retries, health, and shutdown.
5. Evidence: tests, containers, startup, image size, and small runtime comparison.
6. Trade-offs, known gaps, and recommendations for a real migration.
7. Lessons learned: what the six-day timebox revealed about sequencing and risk.

## Definition of Done

The project is ready for review when a reviewer can clone the repository, follow the documented commands, start either implementation, run the shared verification steps, understand the intentional differences, and see evidence that the .NET application was completed before the Go port began.
