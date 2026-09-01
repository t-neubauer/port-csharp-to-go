namespace JobDispatch.Api.Models;

public sealed class CompleteJobRequest
{
    public string? WorkerId { get; set; }

    public string? Worker { get; set; }

    public string? Message { get; set; }

    public object? Result { get; set; }
}
