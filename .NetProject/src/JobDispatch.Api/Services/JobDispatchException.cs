using Microsoft.AspNetCore.Http;

namespace JobDispatch.Api.Services;

public abstract class JobDispatchException : Exception
{
    protected JobDispatchException(string code, string message, int statusCode)
        : base(message)
    {
        Code = code;
        StatusCode = statusCode;
    }

    public string Code { get; }

    public int StatusCode { get; }
}

public sealed class ValidationJobDispatchException : JobDispatchException
{
    public ValidationJobDispatchException(string message)
        : base("VALIDATION_ERROR", message, StatusCodes.Status400BadRequest)
    {
    }
}

public sealed class JobNotFoundException : JobDispatchException
{
    public JobNotFoundException(Guid jobId)
        : base("JOB_NOT_FOUND", $"Job '{jobId}' was not found.", StatusCodes.Status404NotFound)
    {
    }
}

public sealed class InvalidJobStateException : JobDispatchException
{
    public InvalidJobStateException(string message)
        : base("INVALID_JOB_STATE", message, StatusCodes.Status409Conflict)
    {
    }
}
