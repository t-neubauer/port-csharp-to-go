using JobDispatch.Api.Models;

namespace JobDispatch.Api.Services;

public interface IJobService
{
    Task<Job> CreateJobAsync(CreateJobRequest request, CancellationToken cancellationToken = default);

    Task<Job> GetJobAsync(Guid jobId, CancellationToken cancellationToken = default);

    Task<Job> ClaimJobAsync(Guid jobId, ClaimJobRequest request, CancellationToken cancellationToken = default);

    Task<Job> CompleteJobAsync(Guid jobId, string? message = null, CancellationToken cancellationToken = default);

    Task<Job> FailJobAsync(Guid jobId, FailJobRequest request, CancellationToken cancellationToken = default);

    Task<IReadOnlyCollection<Job>> GetEligibleJobsAsync(CancellationToken cancellationToken = default);

    Task<Job> ProcessQueuedJobAsync(Guid jobId, string? workerName = null, CancellationToken cancellationToken = default);
}
