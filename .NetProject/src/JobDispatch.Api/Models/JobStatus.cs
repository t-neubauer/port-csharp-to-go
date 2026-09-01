namespace JobDispatch.Api.Models;

public enum JobStatus
{
    Queued,
    Claimed,
    Completed,
    Failed
}
