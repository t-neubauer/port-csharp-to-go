# .NET Job Dispatch API Requirements Document

Prepared by Prometheus with architecture support from Archie.

## 1. Purpose

This document defines the required behavior of the .NET baseline application that will later be ported to Go. The .NET version is the reference implementation and must behave according to the agreed contract, even before the Go version is built.

The purpose of this baseline is not to create a generic job system, but to provide a stable, testable, and production-shaped API that exercises the migration concerns called out in the project plan.

## 2. Product Summary

The application is a small job dispatch service with the following operating model:

- Clients create jobs.
- Jobs are queued and can later be claimed for processing.
- Workers process claimed jobs.
- Jobs may succeed, fail transiently, or reach terminal failure after retry exhaustion.
- The system exposes both REST endpoints and health probe endpoints.
- Persistent storage and worker lifecycle behavior are part of the required MVP.

## 3. Scope

### In Scope

- HTTP API for job lifecycle operations
- Domain state transitions
- SQL persistence and migrations
- Background worker loop
- Retry policy and lease-based claiming
- Validation and idempotent terminal operations
- Health checks and startup configuration validation
- Structured logs and basic metrics
- Dockerized local runtime
- Test coverage for unit and integration behavior

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
- Claiming must be transactional.
- A claim must carry a lease with expiry.
- A claimed job must not be claimed by another worker until the lease expires.
- Concurrent attempts to claim the same queued job must not produce conflicting state.

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

The service shall use a relational database and migration-based schema management.

Required persistence concerns:

- job table with status, attempt count, max attempts, and timing fields
- claim lease metadata
- schema migration support
- transactional claim behavior
- repository boundary separating persistence concerns from HTTP and domain layers

The schema must include, at minimum:

- `attempt_count`
- `max_attempts`
- `next_attempt_at`
- `lease_owner`
- `lease_expires_at`
- final or terminal timestamps as required by the design

## 8. Configuration and Operations Requirements

### Configuration

The application shall support environment-based configuration with explicit validation.

Required configuration concerns:

- database connection values
- max attempts
- retry backoff values
- worker poll interval
- health endpoint settings
- startup validation for required values

### Observability

The application must emit structured logs with:

- timestamps
- request correlation identifiers
- job identifiers where relevant
- action names
- operational status markers

### Metrics

The application shall expose named counters for:

- created jobs
- claimed jobs
- completed jobs
- retried jobs
- failed jobs

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
- persistence tests for database behavior
- at least one concurrency test for multiple claim attempts

### Acceptance bar

A feature is not complete unless it is verified by tests and manual smoke checks, not merely by compilation.

## 10. Non-Functional Requirements

- The application must be runnable locally with documented commands.
- The service must build into a small container image.
- The container must run as a non-root user.
- The service must remain deterministic enough for repeatable tests.
- The baseline must separate domain behavior from infrastructure concerns in a way that can be ported cleanly to Go.

## 11. Definition of Done for the .NET Baseline

The .NET application will be considered ready for migration when all of the following are true:

- the service runs locally with documented startup steps
- the API contract matches the required endpoints and behavior
- state transitions are enforced by tests
- retry and lease semantics are implemented and verified
- health endpoints respond correctly
- concurrency safeguards are tested
- logs and counters are present
- Docker startup works and runs as non-root
- the behavior is frozen before the Go implementation begins

## 12. Stair-step Implementation Sequence

The work shall proceed in phases:

1. Define API contract and configuration contract.
2. Build the domain and state machine.
3. Add persistence and migrations.
4. Implement web endpoints and validation.
5. Add worker lifecycle and retry logic.
6. Add health, logging, metrics, and shutdown behavior.
7. Validate with unit and integration tests.
8. Freeze the contract for the Go port.

## 13. Success Criteria

The .NET application is successful when it qualifies as a trustworthy baseline for migration:

- behavior is deterministic
- tests exercise the intended contract
- failure and retry semantics are documented and repeatable
- local startup and container execution are straightforward
- the Go implementation can match the same external behavior without redesigning the domain
