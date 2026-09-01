namespace JobDispatch.Api.Services;

public sealed class JobWorkerService : BackgroundService
{
    private readonly IJobService _jobService;
    private readonly JobDispatchOptions _options;
    private readonly ILogger<JobWorkerService> _logger;

    public JobWorkerService(IJobService jobService, JobDispatchOptions options, ILogger<JobWorkerService> logger)
    {
        _jobService = jobService;
        _options = options;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        if (!_options.WorkerEnabled)
        {
            _logger.LogInformation("job_worker_disabled");
            return;
        }

        _logger.LogInformation("job_worker_started poll_interval={PollInterval}", _options.WorkerPollInterval);

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                var jobs = await _jobService.GetEligibleJobsAsync(stoppingToken);
                foreach (var job in jobs)
                {
                    try
                    {
                        await _jobService.ProcessQueuedJobAsync(job.Id, "background-worker", stoppingToken);
                    }
                    catch (JobDispatchException ex)
                    {
                        _logger.LogWarning(ex, "job_worker_failed job_id={JobId} code={Code}", job.Id, ex.Code);
                    }
                }
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "job_worker_iteration_failed");
            }

            await Task.Delay(_options.WorkerPollInterval, stoppingToken);
        }

        _logger.LogInformation("job_worker_stopped");
    }
}
