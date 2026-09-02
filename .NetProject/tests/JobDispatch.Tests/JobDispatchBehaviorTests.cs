using JobDispatch.Api.Models;
using JobDispatch.Api.Services;
using Microsoft.Extensions.Logging.Abstractions;

namespace JobDispatch.Tests;

public class JobDispatchBehaviorTests
{
    [Test]
    public async Task Repository_returns_only_due_retries_and_expired_leases()
    {
        var repository = new InMemoryJobRepository();
        var now = DateTimeOffset.UtcNow;

        await repository.AddAsync(new Job
        {
            Id = Guid.NewGuid(),
            Type = "due",
            Status = JobStatus.Queued,
            CreatedAt = now.AddMinutes(-3),
            NextAttemptAt = now.AddSeconds(-1)
        });
        await repository.AddAsync(new Job
        {
            Id = Guid.NewGuid(),
            Type = "future",
            Status = JobStatus.Queued,
            CreatedAt = now.AddMinutes(-2),
            NextAttemptAt = now.AddMinutes(1)
        });
        await repository.AddAsync(new Job
        {
            Id = Guid.NewGuid(),
            Type = "expired-lease",
            Status = JobStatus.Claimed,
            CreatedAt = now.AddMinutes(-1),
            LeaseExpiresAt = now.AddSeconds(-1)
        });

        var eligible = await repository.GetEligibleJobsAsync(now);

        Assert.That(eligible.Select(job => job.Type), Is.EquivalentTo(new[] { "due", "expired-lease" }));
    }

    [Test]
    public async Task Claim_honors_request_lease_seconds()
    {
        var repository = new InMemoryJobRepository();
        var options = new JobDispatchOptions { LeaseDuration = TimeSpan.FromMinutes(5) };
        var service = new JobService(repository, options, NullLogger<JobService>.Instance);
        var job = await service.CreateJobAsync(new CreateJobRequest { Type = "email" });

        var claimed = await service.ClaimJobAsync(job.Id, new ClaimJobRequest
        {
            LeaseOwner = "worker-a",
            LeaseSeconds = 2
        });

        Assert.That(claimed.LeaseExpiresAt, Is.Not.Null);
        Assert.That(claimed.LeaseExpiresAt.Value - claimed.UpdatedAt, Is.EqualTo(TimeSpan.FromSeconds(2)));
    }

    [Test]
    public async Task Worker_processes_due_jobs_and_stops_on_cancellation()
    {
        var repository = new InMemoryJobRepository();
        var options = new JobDispatchOptions
        {
            WorkerEnabled = true,
            WorkerPollInterval = TimeSpan.FromMilliseconds(10)
        };
        var service = new JobService(repository, options, NullLogger<JobService>.Instance);
        var job = await service.CreateJobAsync(new CreateJobRequest { Type = "email" });
        var worker = new JobWorkerService(service, options, NullLogger<JobWorkerService>.Instance);

        await worker.StartAsync(CancellationToken.None);
        try
        {
            var deadline = DateTime.UtcNow.AddSeconds(2);
            Job? processed = null;
            while (DateTime.UtcNow < deadline)
            {
                processed = await repository.GetByIdAsync(job.Id);
                if (processed?.Status == JobStatus.Completed)
                {
                    break;
                }

                await Task.Delay(10);
            }

            Assert.That(processed?.Status, Is.EqualTo(JobStatus.Completed));
        }
        finally
        {
            await worker.StopAsync(CancellationTokenSource.CreateLinkedTokenSource(
                new CancellationTokenSource(TimeSpan.FromSeconds(2)).Token).Token);
        }
    }

    [Test]
    public void Invalid_options_are_rejected()
    {
        var result = new JobDispatchOptions().Validate(null, new JobDispatchOptions
        {
            DefaultMaxAttempts = 0,
            LeaseDuration = TimeSpan.Zero,
            RetryBackoff = TimeSpan.FromSeconds(-1),
            WorkerPollInterval = TimeSpan.Zero
        });

        Assert.That(result.Failed, Is.True);
        Assert.That(result.Failures, Has.Exactly(4).Items);
    }
}
