using System.Collections.Concurrent;
using JobModel = JobDispatch.Api.Models.Job;
using JobStatusValue = JobDispatch.Api.Models.JobStatus;

namespace JobDispatch.Api.Services;

public sealed class InMemoryJobRepository : IJobRepository
{
    private readonly ConcurrentDictionary<Guid, JobModel> _jobs = new();

    public Task<JobModel> AddAsync(JobModel job, CancellationToken cancellationToken = default)
    {
        _jobs[job.Id] = job;
        return Task.FromResult(job);
    }

    public Task<JobModel?> GetByIdAsync(Guid jobId, CancellationToken cancellationToken = default)
    {
        _jobs.TryGetValue(jobId, out var job);
        return Task.FromResult<JobModel?>(job);
    }

    public Task<JobModel> UpdateAsync(JobModel job, CancellationToken cancellationToken = default)
    {
        _jobs[job.Id] = job;
        return Task.FromResult(job);
    }

    public Task<IReadOnlyCollection<JobModel>> GetEligibleJobsAsync(DateTimeOffset now, CancellationToken cancellationToken = default)
    {
        var jobs = _jobs.Values
            .Where(job => job.Status == JobStatusValue.Queued)
            .OrderBy(job => job.CreatedAt)
            .ToList();

        return Task.FromResult<IReadOnlyCollection<JobModel>>(jobs);
    }
}
