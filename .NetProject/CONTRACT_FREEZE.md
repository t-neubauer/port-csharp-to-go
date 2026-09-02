# Job Dispatch .NET Contract Freeze

Date: 2026-09-02

This document freezes the presentation-focused .NET reference behavior for the Go port. Production infrastructure remains intentionally out of scope.

## HTTP contract

| Method | Route | Success | Important errors |
|---|---|---|---|
| POST | `/jobs` | `201` with the created queued job | `400 VALIDATION_ERROR` |
| GET | `/jobs/{id}` | `200` with the current job | `404 JOB_NOT_FOUND` |
| POST | `/jobs/{id}/claim` | `200` with a claimed job and lease | `404 JOB_NOT_FOUND`, `409 INVALID_JOB_STATE`, `400 VALIDATION_ERROR` |
| POST | `/jobs/{id}/complete` | `200` with a completed job; repeat completion is idempotent | `404 JOB_NOT_FOUND`, `409 INVALID_JOB_STATE` |
| POST | `/jobs/{id}/fail` | `200` with a retry-queued or failed job; repeat terminal failure is idempotent | `404 JOB_NOT_FOUND`, `409 INVALID_JOB_STATE` |
| GET | `/health/live` | `200` with `status: "live"` | None in this baseline |
| GET | `/health/ready` | `200` with `status: "ready"` | None in this baseline |

Error responses have the shape `{ "code": "...", "message": "..." }`. Timestamps are UTC and JSON uses camelCase.

## State and retry contract

- New jobs start as `queued` with attempt count `0`.
- Claims transition `queued` to `claimed` and increment the attempt count.
- A claim has a lease. `leaseSeconds` overrides the configured lease duration when supplied; omitted values use `JobDispatch:LeaseDuration`.
- A live lease prevents another claim. An expired lease makes the job claimable again.
- Completion transitions `claimed` to `completed` and is idempotent for completed jobs.
- A transient failure with attempts remaining transitions `claimed` to `queued` and sets `nextAttemptAt` using the configured retry backoff.
- A non-transient or exhausted failure transitions `claimed` to `failed` and is idempotent for terminal jobs.
- The worker considers queued jobs eligible only when `nextAttemptAt` is absent or due, and claimed jobs eligible when their lease is absent or expired.

The in-memory repository is thread-safe for storage access, but claim is not a transactional compare-and-swap operation. Strong multi-worker atomic claiming is deferred with database persistence.

## Worker and shutdown contract

`JobWorkerService` is an ASP.NET Core `BackgroundService`. When enabled, it polls eligible jobs, invokes the same service workflow used by HTTP, logs per-job failures, and continues after an iteration failure. Cancellation is passed through repository and service calls and stops polling through the host cancellation token. The default worker is disabled so API demonstrations are deterministic unless explicitly enabled.

## Configuration contract

The `JobDispatch` section supports:

```json
{
  "DefaultMaxAttempts": 3,
  "LeaseDuration": "00:05:00",
  "RetryBackoff": "00:00:30",
  "WorkerPollInterval": "00:15:00",
  "WorkerEnabled": false
}
```

Startup validation rejects non-positive max attempts, lease duration, or worker poll interval, and rejects negative retry backoff.

## Known presentation gaps

- Storage is in-memory and process-local.
- There is no database-backed readiness, transactional claiming, authentication, metrics, tracing, Docker packaging, or exactly-once processing.
- Graceful shutdown is demonstrated through host cancellation and worker exit; production drain timeouts and external side-effect coordination are deferred.
