using JobModel = JobDispatch.Api.Models.Job;

namespace JobDispatch.Api.Services;

public interface IJobRepository
{
    Task<JobModel> AddAsync(JobModel job, CancellationToken cancellationToken = default);

    Task<JobModel?> GetByIdAsync(Guid jobId, CancellationToken cancellationToken = default);

    Task<JobModel> UpdateAsync(JobModel job, CancellationToken cancellationToken = default);

    Task<IReadOnlyCollection<JobModel>> GetEligibleJobsAsync(DateTimeOffset now, CancellationToken cancellationToken = default);
}
