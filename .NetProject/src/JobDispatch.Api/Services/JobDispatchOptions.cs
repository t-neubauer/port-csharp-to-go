using Microsoft.Extensions.Options;

namespace JobDispatch.Api.Services;

public sealed class JobDispatchOptions : IValidateOptions<JobDispatchOptions>
{
    public const string SectionName = "JobDispatch";

    public int DefaultMaxAttempts { get; set; } = 3;

    public TimeSpan LeaseDuration { get; set; } = TimeSpan.FromMinutes(5);

    public TimeSpan RetryBackoff { get; set; } = TimeSpan.FromSeconds(30);

    public TimeSpan WorkerPollInterval { get; set; } = TimeSpan.FromMinutes(15);

    public bool WorkerEnabled { get; set; } = false;

    public ValidateOptionsResult Validate(string? name, JobDispatchOptions options)
    {
        var failures = new List<string>();

        if (options.DefaultMaxAttempts <= 0)
        {
            failures.Add($"{nameof(DefaultMaxAttempts)} must be greater than zero.");
        }

        if (options.LeaseDuration <= TimeSpan.Zero)
        {
            failures.Add($"{nameof(LeaseDuration)} must be greater than zero.");
        }

        if (options.RetryBackoff < TimeSpan.Zero)
        {
            failures.Add($"{nameof(RetryBackoff)} must not be negative.");
        }

        if (options.WorkerPollInterval <= TimeSpan.Zero)
        {
            failures.Add($"{nameof(WorkerPollInterval)} must be greater than zero.");
        }

        return failures.Count == 0
            ? ValidateOptionsResult.Success
            : ValidateOptionsResult.Fail(failures);
    }
}
