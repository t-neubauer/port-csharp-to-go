namespace JobDispatch.Api.Models;

public sealed class FailJobRequest
{
    public string? WorkerId { get; set; }

    public string? Error { get; set; }

    public string? Message { get; set; }

    public bool? Transient { get; set; }

    public bool? Retryable { get; set; }
}
