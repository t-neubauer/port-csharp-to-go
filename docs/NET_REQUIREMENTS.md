# .NET Job Dispatch API Requirements Document

Prepared by Prometheus with architecture support from Archie.

## 1. Purpose

This document defines the required behavior of the .NET baseline application that will later be ported to Go. The .NET version is the reference implementation and must behave according to the agreed contract, even before the Go version is built.

The purpose of this baseline is not to create a generic job system, but to provide a small, stable, testable API that demonstrates the most relevant .NET-to-Go migration boundaries in a short presentation.

## 2. Product Summary

The application is a small job dispatch service with the following operating model:

- Clients create jobs.
- Jobs are queued and can later be claimed for processing.
- Workers process claimed jobs.
- Jobs may succeed, fail transiently, or reach terminal failure after retry exhaustion.
- The system exposes both REST endpoints and health probe endpoints.
- In-memory storage and worker lifecycle behavior are part of the presentation scope.

## 3. Scope

### In Scope

- HTTP API for job lifecycle operations
- Domain state transitions
- Repository abstraction with deterministic in-memory storage
- Background worker loop
- Retry policy and lease-based claiming
- Validation and idempotent terminal operations
- Health checks and basic configuration
- Basic structured logs
- Focused unit and HTTP contract tests

### Out of Scope

- Authentication and authorization
- Real external queue infrastructure
- Multi-tenant systems
- Frontend UI
- Kubernetes deployment
- Exactly-once processing guarantees
- Broad distributed tracing and observability platform integration

## 4. Functional Requirements

### 4.1 Job Creation

The system shall allow a client to create a new job.

Required behavior:

- `POST /jobs` accepts a request payload describing the job.
- A new job is created in the queued state.
- The response returns the created job, including its identifier and status.
- The service shall persist the initial timestamp and create audit metadata.

### 4.2 Job Retrieval

The system shall allow retrieval of a single job by identifier.

Required behavior:

- `GET /jobs/{id}` returns the job and current status.
- If the job does not exist, the service returns `404 Not Found`.
- The response format must be consistent across the API.

### 4.3 Job Claiming

The system shall allow a queued job to be claimed for processing.

Required behavior:

- `POST /jobs/{id}/claim` attempts to claim a queued job.
- A claim must carry a lease with expiry.
- A claimed job must not be claimed by another worker until the lease expires.
- The in-memory implementation must behave deterministically for the demonstrated scenarios.

### 4.4 Job Completion

The system shall allow a claimed job to complete successfully.

Required behavior:

- `POST /jobs/{id}/complete` marks a claimed job complete.
- The operation must be idempotent: repeating completion for an already terminally completed job must not corrupt state.
- Completing a non-claimable job must be rejected with a valid domain error.

### 4.5 Job Failure

The system shall allow a claimed job to report a failure.

Required behavior:

- `POST /jobs/{id}/fail` records a failed attempt.
- If the failure is transient and attempts remain, the job is retried with a bounded backoff policy.
- If retry attempts are exhausted, the job becomes terminally failed.
- Repeating failure for an already terminal state must be idempotent.

### 4.6 Worker Processing

The application shall include a hosted background worker.

Required behavior:

- The worker periodically scans for eligible queued jobs.
- The worker claims jobs using the same application logic as the HTTP handler path.
- The worker simulates processing deterministically based on configuration and test inputs.
- The worker respects cancellation and shutdown boundaries.
- In-flight processing must complete or be safely stopped according to shutdown timing.

## 5. Domain Rules

The service shall follow an explicit job state machine.

### Valid states

- `Queued`
- `Claimed`
- `Completed`
- `Failed`

### State transition rules

- `Queued -> Claimed` is valid when a claim succeeds.
- `Claimed -> Completed` is valid when processing succeeds.
- `Claimed -> Queued` is valid only as a retry path after a transient failure and bounded backoff.
- `Claimed -> Failed` is valid only when retry exhaustion occurs.
- Terminal states are not re-entered by normal processing.
- Invalid transitions must produce clear `4xx` or domain errors.

### Retry contract

- Attempts increment when a claim is made.
- Backoff is fixed and documented.
- A job with max attempts reached becomes terminally failed.
- Retry timing is persisted as `next_attempt_at`.

### Lease contract

- A claim includes an expiry timestamp.
- If a worker crashes or stops before completing, a lease eventually expires.
- An expired lease makes the job claimable again.
- The design is at-least-once, not exactly-once.

## 6. API Requirements

### Endpoints

- `POST /jobs`
- `GET /jobs/{id}`
- `POST /jobs/{id}/claim`
- `POST /jobs/{id}/complete`
- `POST /jobs/{id}/fail`
- `GET /health/live`
- `GET /health/ready`

### HTTP contract rules

- Responses must use consistent JSON structure.
- Error responses must include a stable error code and message.
- Unknown jobs must return `404`.
- Invalid payloads and invalid state transitions must return appropriate `4xx` codes.
- UTC timestamps must be used consistently.

## 7. Data and Persistence Requirements

The presentation implementation shall use a repository boundary with deterministic in-memory storage. The boundary must keep domain behavior independent from storage so that a relational implementation can be added later without changing the API or state machine.

## 8. Configuration and Operations Requirements

### Configuration

The application shall support environment-based configuration with explicit validation.

Required configuration concerns:

- max attempts
- retry backoff values
- worker poll interval
- worker enabled/disabled behavior

### Observability

The application must emit structured logs with:

- timestamps
- job identifiers where relevant
- action names
- operational status markers

### Graceful shutdown

The service shall:

- stop accepting new work during shutdown
- allow in-flight processing to finish within a bounded timeout
- exit gracefully without leaving inconsistent state

## 9. Quality Requirements

### Testing

The .NET baseline must include:

- unit tests for state transitions and retry logic
- unit tests for validation and idempotency
- integration tests for the HTTP contract
- repository and worker behavior tests

### Acceptance bar

A feature is not complete unless it is verified by tests and manual smoke checks, not merely by compilation.

## 10. Non-Functional Requirements

- The application must be runnable locally with documented commands.
- The service must remain deterministic enough for repeatable tests.
- The baseline must separate domain behavior from infrastructure concerns in a way that can be ported cleanly to Go.

## 11. Definition of Done for the .NET Baseline

The .NET application will be considered ready for migration when all of the following are true:

- the service runs locally with documented startup steps
- the API contract matches the required endpoints and behavior
- state transitions are enforced by tests
- retry and lease semantics are implemented and verified
- health endpoints respond correctly
- worker cancellation and retry behavior are tested
- basic structured logs are present
- the behavior is frozen before the Go implementation begins

## 12. Stair-step Implementation Sequence

The work shall proceed in phases:

1. Define API contract and configuration contract.
2. Build the domain and state machine.
3. Add the repository boundary and in-memory storage.
4. Implement web endpoints and validation.
5. Add worker lifecycle, retry logic, and cancellation.
6. Add health, basic logging, and shutdown behavior.
7. Validate with unit and HTTP contract tests.
8. Freeze the contract for the Go port.

## 13. Success Criteria

The .NET application is successful when it qualifies as a trustworthy baseline for migration:

- behavior is deterministic
- tests exercise the intended contract
- failure and retry semantics are documented and repeatable
- local startup is straightforward
- the Go implementation can match the same external behavior without redesigning the domain

## CONSIDERATIONS BEYOND PROJECT SCOPE:

The following topics are intentionally excluded from the presentation-focused implementation. They are relevant for a production migration but must not block the .NET baseline or Go port:

- PostgreSQL persistence, schema migrations, and database-backed readiness.
- Transaction-safe concurrent claiming and lease-recovery integration tests.
- Metrics, distributed tracing, correlation middleware, and alerting.
- Docker, Docker Compose, non-root packaging, and image comparisons.
- Authentication, authorization, external queues, Kubernetes, load testing, and exactly-once processing.
