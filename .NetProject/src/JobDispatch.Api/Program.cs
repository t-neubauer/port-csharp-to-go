using System.Text.Json;
using System.Text.Json.Serialization;
using JobDispatch.Api.Models;
using JobDispatch.Api.Services;
using Microsoft.Extensions.Options;

namespace JobDispatch.Api;

public class Program
{
    public static void Main(string[] args)
    {
        var builder = WebApplication.CreateBuilder(args);

        builder.Services.AddOpenApi();

        builder.Services.AddOptions<JobDispatchOptions>()
            .Bind(builder.Configuration.GetSection(JobDispatchOptions.SectionName));
        builder.Services.AddSingleton(sp => sp.GetRequiredService<IOptions<JobDispatchOptions>>().Value);
        builder.Services.AddSingleton<IJobRepository, InMemoryJobRepository>();
        builder.Services.AddSingleton<IJobService, JobService>();
        builder.Services.AddHostedService<JobWorkerService>();

        builder.Services.ConfigureHttpJsonOptions(options =>
        {
            options.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.CamelCase;
            options.SerializerOptions.DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull;
            options.SerializerOptions.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.CamelCase));
        });

        var app = builder.Build();

        if (app.Environment.IsDevelopment())
        {
            app.MapOpenApi();
        }

        app.MapGet("/health/live", () => Results.Ok(new { status = "live", timestamp = DateTimeOffset.UtcNow }));
        app.MapGet("/health/ready", () => Results.Ok(new { status = "ready", timestamp = DateTimeOffset.UtcNow }));

        app.MapPost("/jobs", async (CreateJobRequest request, IJobService jobService) =>
        {
            try
            {
                var created = await jobService.CreateJobAsync(request);
                return Results.Created($"/jobs/{created.Id}", created);
            }
            catch (JobDispatchException ex)
            {
                return ToErrorResult(ex);
            }
        });

        app.MapGet("/jobs/{id:guid}", async (Guid id, IJobService jobService) =>
        {
            try
            {
                var job = await jobService.GetJobAsync(id);
                return Results.Ok(job);
            }
            catch (JobDispatchException ex)
            {
                return ToErrorResult(ex);
            }
        });

        app.MapPost("/jobs/{id:guid}/claim", async (Guid id, ClaimJobRequest request, IJobService jobService) =>
        {
            try
            {
                var claimed = await jobService.ClaimJobAsync(id, request);
                return Results.Ok(claimed);
            }
            catch (JobDispatchException ex)
            {
                return ToErrorResult(ex);
            }
        });

        app.MapPost("/jobs/{id:guid}/complete", async (Guid id, CompleteJobRequest request, IJobService jobService) =>
        {
            try
            {
                var completed = await jobService.CompleteJobAsync(id, request.Message);
                return Results.Ok(completed);
            }
            catch (JobDispatchException ex)
            {
                return ToErrorResult(ex);
            }
        });

        app.MapPost("/jobs/{id:guid}/fail", async (Guid id, FailJobRequest request, IJobService jobService) =>
        {
            try
            {
                var result = await jobService.FailJobAsync(id, request);
                return Results.Ok(result);
            }
            catch (JobDispatchException ex)
            {
                return ToErrorResult(ex);
            }
        });

        app.Run();
    }

    private static IResult ToErrorResult(JobDispatchException ex)
    {
        var response = new ErrorResponse { Code = ex.Code, Message = ex.Message };

        return ex.StatusCode switch
        {
            StatusCodes.Status404NotFound => Results.NotFound(response),
            StatusCodes.Status409Conflict => Results.Conflict(response),
            _ => Results.BadRequest(response)
        };
    }
}
