using System.Text.Json.Serialization;

namespace JobDispatch.Api.Models;

public sealed class Job
{
    public Guid Id { get; set; } = Guid.NewGuid();

    [JsonIgnore]
    public string Type { get; set; } = string.Empty;

    [JsonPropertyName("jobType")]
    public string JobType
    {
        get => Type;
        set => Type = value;
    }

    public string Name { get; set; } = string.Empty;

    public object? Payload { get; set; }

    public JobStatus Status { get; set; } = JobStatus.Queued;

    public int AttemptCount { get; set; }

    public int MaxAttempts { get; set; } = 3;

    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.UtcNow;

    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;

    public DateTimeOffset? NextAttemptAt { get; set; }

    public string? LeaseOwner { get; set; }

    public DateTimeOffset? LeaseExpiresAt { get; set; }

    public DateTimeOffset? CompletedAt { get; set; }

    public DateTimeOffset? FailedAt { get; set; }

    public string? LastError { get; set; }

    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? ErrorCode { get; set; }
}
