namespace JobDispatch.Api.Models;

public sealed class ClaimJobRequest
{
    public string? WorkerId { get; set; }

    public string? LeaseOwner { get; set; }

    public string? Worker { get; set; }

    public int LeaseSeconds { get; set; } = 30;
}
