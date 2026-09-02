# .NET to Go Migration Plan

## Decision Summary

Build the .NET application first as the prerequisite for the migration. The baseline application will be a small **Job Dispatch API**: clients submit jobs, workers claim them, and a hosted background processor retries transient failures and records the final outcome.

This is intentionally more representative than a CRUD sample. It creates a stable behavioral contract that can be ported to Go and compared across runtime behavior, architecture, operational concerns, and developer experience.

**Timebox:** six working days, approximately three focused hours per day, with one developer supported by the Prometheus, Archie, and Devon agents. The presentation-critical path prioritizes a complete in-memory vertical slice; database, migration, and container choices are recorded but do not block the presentation.

## Current Readiness and Open Decisions

The .NET project currently provides a working HTTP and domain reference for the presentation-focused path, but it is not yet a complete implementation of every broader engineering requirement in this plan:

- The repository is currently `InMemoryJobRepository`; relational persistence, schema migrations, and a database-backed readiness check remain to be implemented.
- The worker exists and is disabled by default, but worker behavior needs stronger tests for cancellation, retry timing, and lease recovery.
- The current test suite covers the main HTTP lifecycle and terminal idempotency, but does not yet provide the required persistence or concurrent-claim integration tests.
- Structured job logs are present. Correlation IDs, named metrics, startup validation, Docker/non-root execution, and bounded graceful shutdown still need explicit acceptance evidence.

Therefore, **the Go port must not begin until the presentation-focused .NET contract-freeze checkpoint is completed**. That checkpoint freezes the lifecycle API, state machine, errors, retry/idempotency behavior, worker behavior, health responses, and known gaps. PostgreSQL is a separate stretch track and must not delay the Go port.

The following decisions must be recorded before the database-backed slice is started:

| Decision | Required outcome | Default if not otherwise agreed |
|---|---|---|
| Go version | Pinned toolchain in `go.mod` and documentation | Current supported stable Go version available in the environment |
| Database | Same relational engine for .NET and Go | PostgreSQL |
| Migration tool | One repeatable CLI/library and checked-in migrations | `golang-migrate`-compatible SQL migrations |
| HTTP routing | Standard library or one small pinned router | `net/http` with `http.ServeMux` unless route constraints require otherwise |
| Observability | Structured logger and metrics approach | `log/slog` plus Prometheus-compatible counters |
| Migration ownership | Exactly one component owns startup migrations | Application startup in local MVP; document how production would differ |
| Worker model | One in-process worker loop for MVP | One worker per service instance, at-least-once processing |

If a default is used, record it in the migration effort log and keep the choice consistent in both implementations.

### Confirmed technology decisions

The initial decisions for this migration are:

- **Database:** PostgreSQL.
- **Migration format/tool:** `golang-migrate`-compatible SQL migrations, used consistently by both implementations.
- **Go HTTP routing:** standard library `net/http` with `http.ServeMux`.
- **Observability:** `log/slog` plus Prometheus-compatible counters.
- **Migration ownership:** the API application runs pending migrations during startup. Readiness remains unhealthy until the database is reachable and migrations have completed.
- **Go version:** use the current stable version available in the environment and pin it in `go.mod` when the Go project is created.

The application-startup migration choice is intended to keep the local MVP easy to run with one command. A production deployment may later move migrations to a separate release step; that is an operational distinction and must not change the schema or API contract.

## Objectives

- Deliver a runnable, tested .NET baseline before porting.
- Port the same externally observable behavior to Go without redesigning the domain.
- Compare equivalent production concerns, not just syntax and HTTP routing.
- Produce concise evidence for a technical presentation: focused tests, startup/smoke behavior, and documented trade-offs.

## Presentation Scope Assessment

The engineering plan is broader than what can be explained meaningfully in a 5–10 minute presentation. The presentation should tell one coherent story, not demonstrate every planned production feature.

The recommended presentation thesis is:

> A small job-dispatch service exposes the important .NET-to-Go migration boundaries: HTTP contracts, domain state transitions, dependency composition, error handling, cancellation, and background work. Go preserves the behavior while making several runtime responsibilities more explicit.

The finished project should therefore optimize for **one understandable vertical slice and a few well-supported migration lessons**, rather than maximum infrastructure coverage. A reviewer will learn more from seeing one complete create → claim → process → retry/complete flow and its Go/.NET mapping than from seeing a partially implemented database, metrics, tracing, deployment, and performance story.

### What can realistically fit in the presentation

| Topic | Presentation treatment | Target time |
|---|---|---:|
| Motivation and AI-assisted approach | Why the migration was selected, how planning and review were used | 1 minute |
| .NET reference | One architecture diagram and the job state machine | 1–1.5 minutes |
| Go port | Package layout and one end-to-end request/worker flow | 1.5–2 minutes |
| Key migration challenges | Three focused comparisons: DI/composition, errors, and cancellation/concurrency | 2–3 minutes |
| Demonstration/evidence | One smoke flow, tests, and one intentional difference | 1–1.5 minutes |
| Broader considerations and conclusion | What a real migration would still need | 1 minute |

At ten minutes, this allows roughly 6–8 slides plus a short demo. At five minutes, the demo and detailed implementation comparison must be reduced to one representative flow.

### Presentation-focused scope

Keep these items as the implementation target:

- The five job lifecycle endpoints and the two health endpoints.
- An explicit state machine with validation and stable error responses.
- A repository interface with a deterministic in-memory implementation first.
- The background worker, cancellation, deterministic success/failure behavior, retry, and terminal idempotency.
- Explicit Go composition from `main`, `struct` models, interfaces, returned errors, and `context.Context`.
- Focused unit and HTTP contract tests showing behavior parity.
- A simple runnable Go command and, if time permits, one small non-root container.
- `docs/MIGRATION_EFFORTS.md` entries for each completed slice, written as teaching material.

### Defer or cut from the presentation-critical path

These are valuable production topics but should not block the presentation:

- PostgreSQL implementation, schema migrations, and database-backed readiness.
- Full transactional/concurrent database claiming and lease-recovery integration tests.
- Prometheus-compatible metrics and distributed tracing.
- Extensive correlation middleware and production logging pipelines.
- Docker Compose orchestration, image-size comparisons, and deployment concerns.
- Performance benchmarking or claims about runtime superiority.
- Kubernetes, authentication, external queues, load testing, and exactly-once processing.

The database and operational items should remain in the plan as **future migration considerations**. If the implementation is completed early, use one of them as a single stretch demonstration; do not attempt to present all of them.

### Revised definition of presentation-ready

For the short presentation, the project is ready when:

1. The .NET and Go services demonstrate the same lifecycle contract.
2. The Go port has at least one clearly explained idiomatic difference from .NET.
3. Tests or a repeatable smoke script provide evidence for the demonstrated behavior.
4. The migration log explains the implementation in language suitable for a Go beginner.
5. Deferred production work is clearly labeled as deferred, not implied to be complete.

The broader MVP definition later in this document remains the long-form engineering target, but it must not force the presentation project into an unfinished breadth-first implementation. If schedule pressure appears, preserve the presentation-focused scope and move broader items to stretch work.

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

## Broader Engineering MVP Acceptance Criteria

This section describes the fuller production-shaped target. It is intentionally broader than the presentation-ready target below and should be treated as optional stretch work when the six-day timebox is used for a short presentation.

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

## Presentation-Ready Acceptance Criteria

The presentation-critical implementation is complete when both implementations:

- Expose the five job operations and the two health endpoints.
- Demonstrate the same create, claim, complete, fail, retry, and terminal-idempotency behavior.
- Enforce the explicit state machine and return stable, understandable error responses.
- Include a deterministic worker path with cancellation and bounded retry behavior.
- Have focused unit and HTTP contract tests for the behavior being demonstrated.
- Can be started with documented local commands.
- Have migration log entries explaining the .NET-to-Go mapping for the demonstrated slices.
- Clearly label PostgreSQL, metrics, deployment, performance, and other production concerns as deferred or stretch work.

The short presentation does not require a database-backed implementation, full production observability, Docker Compose, or performance claims.

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
- Create the .NET solution, projects, dependency composition, and health endpoints. Add container setup only on the broader engineering track.
- Add a minimal vertical slice: create and read a job.
- Create `docs/MIGRATION_EFFORTS.md` and record each decision, completed slice, parity evidence, and intentional difference as work happens.

**Checkpoint:** the .NET service starts locally, its contract is written down, and create/read work end to end.

### Day 2: Complete the .NET domain path

**Goal:** finish the behavior that Go must reproduce.

- Implement claim, complete, and fail transitions.
- Add the relational schema and migrations only if pursuing the broader engineering MVP.
- For the presentation path, make the in-memory repository behavior safe and deterministic for the demonstrated scenarios.
- Add validation, consistent errors, idempotency rules, and repository tests.

**Checkpoint:** .NET API and focused domain tests pass; the state machine is stable enough to port. Database persistence is a separate stretch checkpoint.

### Day 3: Make the .NET baseline production-shaped

**Goal:** finish operational behavior and capture the reference evidence.

- Add the hosted worker, cancellation, bounded retry/backoff, and graceful shutdown.
- Add structured logging, correlation IDs, and counters for created, claimed, completed, retried, and failed jobs.
- Add liveness/readiness checks, focused configuration validation, worker tests, and graceful shutdown evidence for the presentation path.
- Add integration tests, database readiness, and a non-root Docker image only if pursuing the broader engineering MVP.
- Record the test count and smoke-test transcript. Record build time, image size, and database evidence only when those features are implemented.
- Run the presentation contract-freeze checklist: API examples, error matrix, state-transition table, retry timing, worker behavior, shutdown semantics, and known gaps. Add the database schema and configuration table when the broader persistence track is selected.

**Checkpoint:** the .NET implementation meets the presentation-ready criteria or the user explicitly approves a short, visible gap list. Freeze its API and behavior before porting.

### Day 4: Port the core to Go

**Goal:** reproduce the contract and domain behavior first.

- Create the Go module and map the .NET boundaries to idiomatic Go packages.
- Implement configuration, HTTP handlers, validation, domain state transitions, in-memory repository, and health endpoints.
- Port focused unit and HTTP contract tests alongside the implementation.
- Add PostgreSQL, migrations, and database integration tests only on the broader engineering track.
- Implement one vertical slice at a time: create/read, claim, complete/fail, then persistence and worker integration. Keep the corresponding .NET and Go examples adjacent in the migration log.

**Scope rule:** for the presentation-focused path, implement the in-memory repository before any PostgreSQL work. PostgreSQL is optional stretch work and must not delay the first complete Go lifecycle flow.

**Checkpoint:** Go create/read/claim/complete/fail flows pass the same contract cases as .NET.

### Day 5: Port operations and close parity gaps

**Goal:** make Go runnable and operationally comparable.

- Port the worker, cancellation, retry policy, idempotency, basic structured logs, and health behavior.
- Execute the shared lifecycle smoke test against both services.
- Build and run the Go container, database-backed readiness, and concurrent database claim test only on the broader engineering track.
- Resolve only parity defects and record intentional differences with reasons.

**Scope rule:** prioritize worker cancellation, retry/idempotency behavior, and a repeatable smoke test. Metrics, database integration, and container comparisons are stretch work.

**Checkpoint:** both implementations pass the presentation-ready acceptance suite and can be started from documented commands.

### Day 6: Compare, document, and present

**Goal:** turn implementation work into defensible project results.

- Run a small repeatable comparison: test result, startup behavior, and one representative smoke scenario. Add build duration and image size only if the corresponding container work was completed.
- Review code and document .NET-to-Go mappings, trade-offs, risks, and remaining production work.
- Prepare the presentation and demo script: baseline problem, architecture, porting decisions, evidence, parity gaps, and lessons learned.
- Reserve the final hour for fixing only release-blocking issues.

**Exit checkpoint:** reproducible demo, passing presentation-ready checks, comparison notes, and a clear list of deferred engineering work.

## Sequential Go-Migration Guide

Complete these steps in order. Do not start the next step while the previous step has an unresolved contract or parity failure.

1. **Freeze the reference.** Verify the .NET endpoint, JSON, status-code, error, state, retry, lease, timestamp, configuration, health, shutdown, and worker contracts. Mark every unsupported requirement as either fixed before porting or an approved parity gap.
2. **Create the Go module.** Add the pinned Go toolchain, module path, formatting rules, local run command, and a minimal executable that starts and stops cleanly.
3. **Build the domain vocabulary.** Port `Job`, status constants, request/response DTOs, validation rules, domain errors, and retry policy without adding Go-specific behavior changes.
4. **Build configuration and composition.** Parse environment variables, apply defaults, validate required values, construct dependencies explicitly, and make ownership/lifecycle visible from `main`.
5. **Implement the repository boundary.** Start with the repository interface, deterministic in-memory implementation, and tests. Add the relational schema, migrations, CRUD operations, and transaction-safe claim with lease expiry only on the broader engineering track.
6. **Implement HTTP behavior.** Add routing, JSON decoding/encoding, validation, error mapping, request correlation, and the seven documented endpoints. Match the frozen .NET contract before improving ergonomics.
7. **Port the worker.** Use `context.Context` for cancellation, a ticker for polling, the same service methods as HTTP, deterministic processing, bounded retries, and a bounded shutdown wait.
8. **Add operational behavior.** Implement liveness/readiness dependency checks, structured logs, counters, startup failure handling, and graceful server shutdown.
9. **Add delivery artifacts.** Document the local run command for both implementations. Create a multi-stage non-root Docker image and database startup command only if pursuing the broader engineering track.
10. **Run parity checks.** Execute shared contract, retry, idempotency, worker, health, shutdown, and smoke checks for the presentation path. Add persistence, lease-recovery, concurrent-claim, and container checks only on the broader engineering track.
11. **Document differences.** For every intentional difference, explain the .NET mechanism, the Go mechanism, why the difference is idiomatic or necessary, and how equivalent behavior was verified.
12. **Close and publish.** Resolve release-blocking gaps, retain approved differences, complete the comparison evidence, and update the definition-of-done checklist.

## Proposed Go Project Layout

```text
GoProject/
├── go.mod
├── go.sum
├── README.md
├── Dockerfile
├── docker-compose.yml
├── cmd/
│   └── jobdispatch/
│       └── main.go                 # Process entrypoint and lifecycle wiring
├── internal/
│   ├── config/
│   │   ├── config.go               # Environment parsing and validation
│   │   └── config_test.go
│   ├── domain/
│   │   ├── job.go                  # Job model and status values
│   │   ├── errors.go               # Stable domain errors and HTTP mapping data
│   │   ├── retry.go                # Backoff and retry decisions
│   │   └── job_test.go
│   ├── service/
│   │   ├── job_service.go          # Shared lifecycle operations
│   │   ├── processor.go            # Deterministic worker processing
│   │   └── service_test.go
│   ├── repository/
│   │   ├── repository.go            # Persistence interface
│   │   ├── postgres.go              # SQL implementation and transactions
│   │   └── postgres_test.go
│   ├── httpapi/
│   │   ├── handler.go               # Endpoint handlers
│   │   ├── middleware.go            # Correlation and request concerns
│   │   ├── errors.go                # Domain-to-HTTP response mapping
│   │   └── handler_test.go
│   ├── worker/
│   │   ├── worker.go                # Polling, cancellation, and shutdown
│   │   └── worker_test.go
│   └── health/
│       └── health.go                # Liveness and readiness checks
├── migrations/
│   ├── 000001_create_jobs.up.sql
│   └── 000001_create_jobs.down.sql
└── tests/
    ├── contract/                    # Shared HTTP behavior checks
    └── integration/                 # Database and concurrency scenarios
```

Keep application-only packages under `internal/` until there is a demonstrated need to publish them. The layout is a guide, not a reason to create empty abstractions: a file should exist when it owns a real boundary or testable behavior.

## Migration Documentation Rules

`docs/MIGRATION_EFFORTS.md` is the running implementation record. Each completed feature should include:

1. The .NET source files and behavior being ported.
2. The Go files and beginner-oriented explanation of the implementation.
3. The important language/runtime difference, including error handling, dependency wiring, concurrency, cancellation, and data access.
4. The parity evidence: test name, command, smoke example, or explicitly approved gap.
5. Any new Go vocabulary introduced, explained in plain language on first use.

The log should explain concepts before comparing syntax. It should never assume that a reader already knows goroutines, channels, interfaces, pointers, contexts, or Go package visibility.

## Architecture and Porting Map

| Concern | .NET baseline | Go target | Comparison focus |
|---|---|---|---|
| HTTP | ASP.NET Core endpoints/controllers | `net/http` with a small router | Binding, middleware, status/error behavior |
| Composition | Built-in dependency injection and hosted service | Explicit constructors and lifecycle-managed worker | Implicit versus explicit dependency wiring |
| Domain | C# records/enums and service layer | Go structs, constants, and service package | Modeling, error flow, and test ergonomics |
| Persistence | In-memory reference; SQL is broader target | In-memory parity first; SQL is optional stretch | Explicit boundary now, transactions and resource ownership later |
| Background work | `BackgroundService` and cancellation tokens | Goroutine plus context cancellation | Shutdown semantics and concurrency clarity |
| Configuration | Options binding and startup validation | Environment parser with explicit validation | Defaults, failure timing, and discoverability |
| Observability | Structured logging plus correlation middleware | Structured logger plus request middleware | Context propagation and useful fields |
| Health | ASP.NET Core health checks | Explicit liveness/readiness handlers | Dependency checks and orchestration semantics |
| Delivery | Documented local run; container is optional stretch | Documented local run; container is optional stretch | Startup parity first, image comparison only if implemented |

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
| Database setup consumes the schedule | Keep the in-memory repository as the presentation baseline; add PostgreSQL only as stretch work. |
| “Production-level” becomes too broad | Treat health, shutdown, config, retries, tests, and the worker as the fixed presentation slice; defer databases, containers, metrics, auth, and Kubernetes. |
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
4. Migration challenges: dependency composition, error handling, worker lifecycle, retries, health, and shutdown.
5. Evidence: focused tests, startup/smoke behavior, and one intentional difference; mention database and deployment as deferred production work.
6. Trade-offs, known gaps, and recommendations for a real migration.
7. Lessons learned: what the six-day timebox revealed about sequencing and risk.

## Presentation Definition of Done

The project is ready for the short presentation when a reviewer can clone the repository, follow the documented local commands, start either implementation, run the focused lifecycle verification steps, understand the three intentional .NET-to-Go differences, and see evidence that the .NET behavior was frozen before the Go port began. Broader persistence, deployment, and observability work must be listed separately as deferred engineering considerations.
