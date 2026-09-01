using System.Text.Json.Serialization;

namespace JobDispatch.Api.Models;

public sealed class CreateJobRequest
{
    public string? Type { get; set; }

    public string? Name { get; set; }

    [JsonPropertyName("jobType")]
    public string? JobType
    {
        get => Type;
        set => Type = value;
    }

    public object? Payload { get; set; }

    public object? Data { get; set; }

    public object? Body { get; set; }

    [JsonPropertyName("maxAttempts")]
    public int? MaxAttempts { get; set; }

    [JsonIgnore]
    public int? Max_Attempts
    {
        get => MaxAttempts;
        set => MaxAttempts = value ?? MaxAttempts;
    }
}
