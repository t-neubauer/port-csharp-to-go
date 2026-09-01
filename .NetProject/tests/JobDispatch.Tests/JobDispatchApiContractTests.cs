using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using JobDispatch.Api;
using Microsoft.AspNetCore.Mvc.Testing;

namespace JobDispatch.Tests;

public class JobDispatchApiContractTests
{
    private WebApplicationFactory<Program> _factory = null!;
    private HttpClient _client = null!;

    [SetUp]
    public void Setup()
    {
        _factory = new WebApplicationFactory<Program>();
        _client = _factory.CreateClient();
    }

    [TearDown]
    public void TearDown()
    {
        _client.Dispose();
        _factory.Dispose();
    }

    [Test]
    public async Task Create_and_get_job_round_trip_returns_expected_contract()
    {
        var created = await CreateJobAsync();

        Assert.That(created.GetProperty("status").GetString(), Is.EqualTo("queued"));
        Assert.That(created.GetProperty("id").GetGuid(), Is.Not.EqualTo(Guid.Empty));
        Assert.That(created.GetProperty("attemptCount").GetInt32(), Is.EqualTo(0));
        Assert.That(created.GetProperty("maxAttempts").GetInt32(), Is.GreaterThan(0));

        var jobId = created.GetProperty("id").GetGuid();
        Console.WriteLine($"created job: {created}");
        var getResponse = await _client.GetAsync($"/jobs/{jobId}");
        Console.WriteLine($"get-job status={getResponse.StatusCode} body={await getResponse.Content.ReadAsStringAsync()}");
        Assert.That(getResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var readDoc = JsonDocument.Parse(await getResponse.Content.ReadAsStringAsync());
        var read = readDoc.RootElement;

        Assert.That(read.GetProperty("id").GetGuid(), Is.EqualTo(created.GetProperty("id").GetGuid()));
        Assert.That(read.GetProperty("status").GetString(), Is.EqualTo("queued"));
        Assert.That(read.GetProperty("jobType").GetString(), Is.EqualTo("email"));
    }

    [Test]
    public async Task Claim_complete_and_fail_transitions_follow_the_domain_state_machine()
    {
        var jobId = (await CreateJobAsync()).GetProperty("id").GetGuid();

        var claimResponse = await _client.PostAsJsonAsync($"/jobs/{jobId}/claim", new { leaseOwner = "worker-a" });
        Assert.That(claimResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var claimDoc = JsonDocument.Parse(await claimResponse.Content.ReadAsStringAsync());
        var claimed = claimDoc.RootElement;
        Assert.That(claimed.GetProperty("status").GetString(), Is.EqualTo("claimed"));
        Assert.That(claimed.GetProperty("attemptCount").GetInt32(), Is.EqualTo(1));
        Assert.That(claimed.GetProperty("leaseOwner").GetString(), Is.EqualTo("worker-a"));

        var secondClaim = await _client.PostAsJsonAsync($"/jobs/{jobId}/claim", new { leaseOwner = "worker-b" });
        Assert.That(secondClaim.StatusCode, Is.EqualTo(HttpStatusCode.Conflict));

        var completeResponse = await _client.PostAsJsonAsync($"/jobs/{jobId}/complete", new { message = "completed by client" });
        Assert.That(completeResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var completeDoc = JsonDocument.Parse(await completeResponse.Content.ReadAsStringAsync());
        var completed = completeDoc.RootElement;
        Assert.That(completed.GetProperty("status").GetString(), Is.EqualTo("completed"));

        var retryableJobId = (await CreateJobAsync(maxAttempts: 3)).GetProperty("id").GetGuid();
        var retryClaimResponse = await _client.PostAsJsonAsync($"/jobs/{retryableJobId}/claim", new { leaseOwner = "worker-c" });
        Assert.That(retryClaimResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        var failResponse = await _client.PostAsJsonAsync($"/jobs/{retryableJobId}/fail", new { error = "temporary outage", transient = true });
        Assert.That(failResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var failDoc = JsonDocument.Parse(await failResponse.Content.ReadAsStringAsync());
        var failed = failDoc.RootElement;
        Assert.That(failed.GetProperty("status").GetString(), Is.EqualTo("queued"));
        Assert.That(failed.GetProperty("attemptCount").GetInt32(), Is.EqualTo(1));
        Assert.That(failed.TryGetProperty("nextAttemptAt", out _), Is.True);
    }

    [Test]
    public async Task Retry_and_terminal_idempotency_do_not_corrupt_state()
    {
        var jobId = (await CreateJobAsync(maxAttempts: 1)).GetProperty("id").GetGuid();

        var claim = await _client.PostAsJsonAsync($"/jobs/{jobId}/claim", new { leaseOwner = "worker-d" });
        Assert.That(claim.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        var failResponse = await _client.PostAsJsonAsync($"/jobs/{jobId}/fail", new { error = "terminal failure", transient = true });
        Assert.That(failResponse.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var failDoc = JsonDocument.Parse(await failResponse.Content.ReadAsStringAsync());
        var failed = failDoc.RootElement;
        Assert.That(failed.GetProperty("status").GetString(), Is.EqualTo("failed"));
        Assert.That(failed.GetProperty("attemptCount").GetInt32(), Is.EqualTo(1));

        var duplicateFail = await _client.PostAsJsonAsync($"/jobs/{jobId}/fail", new { error = "terminal failure", transient = true });
        Assert.That(duplicateFail.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        using var duplicateDoc = JsonDocument.Parse(await duplicateFail.Content.ReadAsStringAsync());
        var duplicate = duplicateDoc.RootElement;
        Assert.That(duplicate.GetProperty("status").GetString(), Is.EqualTo("failed"));
        Assert.That(duplicate.GetProperty("attemptCount").GetInt32(), Is.EqualTo(1));
    }

    [Test]
    public async Task Health_endpoints_return_healthy_responses_and_invalid_transitions_are_rejected()
    {
        var live = await _client.GetAsync("/health/live");
        Assert.That(live.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        var ready = await _client.GetAsync("/health/ready");
        Assert.That(ready.StatusCode, Is.EqualTo(HttpStatusCode.OK));

        var invalidJobId = (await CreateJobAsync()).GetProperty("id").GetGuid();
        var invalidResponse = await _client.PostAsJsonAsync($"/jobs/{invalidJobId}/complete", new { message = "should fail" });
        Assert.That(invalidResponse.StatusCode, Is.EqualTo(HttpStatusCode.Conflict));

        var notFound = await _client.GetAsync($"/jobs/{Guid.NewGuid()}");
        Assert.That(notFound.StatusCode, Is.EqualTo(HttpStatusCode.NotFound));
    }

    private async Task<JsonElement> CreateJobAsync(int maxAttempts = 3)
    {
        var response = await _client.PostAsJsonAsync("/jobs", new
        {
            jobType = "email",
            maxAttempts = maxAttempts,
            payload = new { to = "ops@example.com", template = "welcome" }
        });

        Assert.That(response.StatusCode, Is.EqualTo(HttpStatusCode.Created), $"Create job failed: {response.StatusCode} {await response.Content.ReadAsStringAsync()}");
        using var document = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        return document.RootElement.Clone();
    }

}
