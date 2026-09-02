using JobDispatch.Api.Models;

namespace JobDispatch.Api.Services;

public sealed class JobService : IJobService
{
    private readonly IJobRepository _repository;
    private readonly JobDispatchOptions _options;
    private readonly ILogger<JobService> _logger;

    public JobService(IJobRepository repository, JobDispatchOptions options, ILogger<JobService> logger)
    {
        _repository = repository;
        _options = options;
        _logger = logger;
    }

    public async Task<Job> CreateJobAsync(CreateJobRequest request, CancellationToken cancellationToken = default)
    {
        if (request is null)
        {
            throw new ValidationJobDispatchException("Request body is required.");
        }

        var type = GetRequiredType(request);
        var maxAttempts = ResolveMaxAttempts(request, _options.DefaultMaxAttempts);

        var job = new Job
        {
            Id = Guid.NewGuid(),
            Type = type,
            Name = string.IsNullOrWhiteSpace(request.Name) ? type : request.Name,
            Payload = ResolvePayload(request),
            MaxAttempts = maxAttempts,
            Status = JobStatus.Queued,
            CreatedAt = DateTimeOffset.UtcNow,
            UpdatedAt = DateTimeOffset.UtcNow,
            NextAttemptAt = DateTimeOffset.UtcNow
        };

        await _repository.AddAsync(job, cancellationToken);

        _logger.LogInformation("job_created job_id={JobId} type={Type} max_attempts={MaxAttempts}", job.Id, job.Type, job.MaxAttempts);
        return job;
    }

    public async Task<Job> GetJobAsync(Guid jobId, CancellationToken cancellationToken = default)
    {
        var job = await _repository.GetByIdAsync(jobId, cancellationToken);
        if (job is null)
        {
            throw new JobNotFoundException(jobId);
        }

        return job;
    }

    public async Task<Job> ClaimJobAsync(Guid jobId, ClaimJobRequest request, CancellationToken cancellationToken = default)
    {
        var job = await GetJobAsync(jobId, cancellationToken);
        var now = DateTimeOffset.UtcNow;
        var leaseOwner = string.IsNullOrWhiteSpace(request?.LeaseOwner) ? (string.IsNullOrWhiteSpace(request?.WorkerId) ? (string.IsNullOrWhiteSpace(request?.Worker) ? "worker" : request.Worker) : request.WorkerId) : request.LeaseOwner;
        var leaseDuration = request?.LeaseSeconds is null
            ? _options.LeaseDuration
            : TimeSpan.FromSeconds(request.LeaseSeconds.Value);

        if (leaseDuration <= TimeSpan.Zero)
        {
            throw new ValidationJobDispatchException("lease_seconds must be greater than zero.");
        }

        if (job.Status == JobStatus.Completed)
        {
            throw new InvalidJobStateException($"Job '{jobId}' is already completed.");
        }

        if (job.Status == JobStatus.Failed)
        {
            throw new InvalidJobStateException($"Job '{jobId}' is already failed.");
        }

        if (job.Status == JobStatus.Claimed && job.LeaseExpiresAt.HasValue && job.LeaseExpiresAt.Value > now)
        {
            throw new InvalidJobStateException($"Job '{jobId}' is already claimed until {job.LeaseExpiresAt:O}.");
        }

        if (job.Status == JobStatus.Queued || (job.Status == JobStatus.Claimed && (!job.LeaseExpiresAt.HasValue || job.LeaseExpiresAt.Value <= now)))
        {
            job.Status = JobStatus.Claimed;
            job.AttemptCount += 1;
            job.LeaseOwner = leaseOwner;
            job.LeaseExpiresAt = now.Add(leaseDuration);
            job.NextAttemptAt = null;
            job.UpdatedAt = now;
            await _repository.UpdateAsync(job, cancellationToken);

            _logger.LogInformation("job_claimed job_id={JobId} lease_owner={LeaseOwner} attempt_count={AttemptCount}", job.Id, job.LeaseOwner, job.AttemptCount);
            return job;
        }

        throw new InvalidJobStateException($"Job '{jobId}' cannot be claimed from status '{job.Status}'.");
    }

    public async Task<Job> CompleteJobAsync(Guid jobId, string? message = null, CancellationToken cancellationToken = default)
    {
        var job = await GetJobAsync(jobId, cancellationToken);
        var now = DateTimeOffset.UtcNow;

        if (job.Status == JobStatus.Completed)
        {
            return job;
        }

        if (job.Status != JobStatus.Claimed)
        {
            throw new InvalidJobStateException($"Job '{jobId}' must be claimed before it can be completed.");
        }

        job.Status = JobStatus.Completed;
        job.CompletedAt = now;
        job.UpdatedAt = now;
        job.LeaseOwner = null;
        job.LeaseExpiresAt = null;
        job.NextAttemptAt = null;
        job.LastError = message;

        await _repository.UpdateAsync(job, cancellationToken);

        _logger.LogInformation("job_completed job_id={JobId}", job.Id);
        return job;
    }

    public async Task<Job> FailJobAsync(Guid jobId, FailJobRequest request, CancellationToken cancellationToken = default)
    {
        var job = await GetJobAsync(jobId, cancellationToken);
        var now = DateTimeOffset.UtcNow;

        if (job.Status == JobStatus.Completed)
        {
            return job;
        }

        if (job.Status == JobStatus.Failed)
        {
            return job;
        }

        if (job.Status != JobStatus.Claimed)
        {
            throw new InvalidJobStateException($"Job '{jobId}' must be claimed before it can fail.");
        }

        var errorMessage = !string.IsNullOrWhiteSpace(request?.Error) ? request.Error : (!string.IsNullOrWhiteSpace(request?.Message) ? request.Message : "Job processing failed.");
        var transient = request is not null && (request.Transient ?? request.Retryable ?? false);
        var shouldRetry = transient && job.AttemptCount < job.MaxAttempts;

        if (shouldRetry)
        {
            job.Status = JobStatus.Queued;
            job.LeaseOwner = null;
            job.LeaseExpiresAt = null;
            job.LastError = errorMessage;
            job.NextAttemptAt = now.Add(_options.RetryBackoff);
            job.UpdatedAt = now;

            await _repository.UpdateAsync(job, cancellationToken);

            _logger.LogInformation("job_retry_scheduled job_id={JobId} retry_at={RetryAt}", job.Id, job.NextAttemptAt);
            return job;
        }

        job.Status = JobStatus.Failed;
        job.FailedAt = now;
        job.UpdatedAt = now;
        job.LeaseOwner = null;
        job.LeaseExpiresAt = null;
        job.NextAttemptAt = null;
        job.LastError = errorMessage;
        job.ErrorCode = "JOB_FAILED";

        await _repository.UpdateAsync(job, cancellationToken);

        _logger.LogInformation("job_failed job_id={JobId} error={Error}", job.Id, errorMessage);
        return job;
    }

    public async Task<IReadOnlyCollection<Job>> GetEligibleJobsAsync(CancellationToken cancellationToken = default)
    {
        return await _repository.GetEligibleJobsAsync(DateTimeOffset.UtcNow, cancellationToken);
    }

    public async Task<Job> ProcessQueuedJobAsync(Guid jobId, string? workerName = null, CancellationToken cancellationToken = default)
    {
        var job = await GetJobAsync(jobId, cancellationToken);

        if (job.Status == JobStatus.Completed || job.Status == JobStatus.Failed)
        {
            return job;
        }

        if (job.Status == JobStatus.Queued && job.NextAttemptAt.HasValue && job.NextAttemptAt.Value > DateTimeOffset.UtcNow)
        {
            return job;
        }

        var claimant = string.IsNullOrWhiteSpace(workerName) ? "background-worker" : workerName;
        var claimed = await ClaimJobAsync(jobId, new ClaimJobRequest { WorkerId = claimant, LeaseOwner = claimant, Worker = claimant }, cancellationToken);

        var shouldRetry = claimed.Type.Contains("fail", StringComparison.OrdinalIgnoreCase) || claimed.Name.Contains("fail", StringComparison.OrdinalIgnoreCase);
        if (shouldRetry)
        {
            return await FailJobAsync(jobId, new FailJobRequest { Error = "Worker failure simulated.", Transient = true }, cancellationToken);
        }

        if (claimed.Type.Contains("retry", StringComparison.OrdinalIgnoreCase) || claimed.Name.Contains("retry", StringComparison.OrdinalIgnoreCase))
        {
            return await FailJobAsync(jobId, new FailJobRequest { Error = "Transient retry required.", Transient = true }, cancellationToken);
        }

        return await CompleteJobAsync(jobId, "processed by worker", cancellationToken);
    }

    private static string GetRequiredType(CreateJobRequest request)
    {
        var type = request.Type ?? request.JobType ?? request.Name;
        if (string.IsNullOrWhiteSpace(type))
        {
            throw new ValidationJobDispatchException("Job type is required.");
        }

        return type.Trim();
    }

    private static object ResolvePayload(CreateJobRequest request)
    {
        if (request.Payload is not null)
        {
            return request.Payload;
        }

        if (request.Data is not null)
        {
            return request.Data;
        }

        if (request.Body is not null)
        {
            return request.Body;
        }

        return new { };
    }

    private static int ResolveMaxAttempts(CreateJobRequest request, int defaultMaxAttempts)
    {
        var candidate = request.MaxAttempts ?? request.Max_Attempts ?? defaultMaxAttempts;
        if (candidate <= 0)
        {
            throw new ValidationJobDispatchException("max_attempts must be greater than zero.");
        }

        return candidate;
    }
}
