namespace JobDispatch.Api.Services;

public sealed class JobDispatchOptions
{
    public const string SectionName = "JobDispatch";

    public int DefaultMaxAttempts { get; set; } = 3;

    public TimeSpan LeaseDuration { get; set; } = TimeSpan.FromMinutes(5);

    public TimeSpan RetryBackoff { get; set; } = TimeSpan.FromSeconds(30);

    public TimeSpan WorkerPollInterval { get; set; } = TimeSpan.FromMinutes(15);

    public bool WorkerEnabled { get; set; } = false;
}
