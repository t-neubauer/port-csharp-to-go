# This script verifies the presentation-critical process contract from outside
# the application: build, start, health check, create, claim, complete, stop.

$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")

# Use a temporary local address so the smoke test does not depend on defaults
# or conflict with another service using the standard development port.
$env:JOBDISPATCH_ADDR = "127.0.0.1:18080"
$env:JobDispatch__WorkerEnabled = "false"

$executable = Join-Path (Get-Location) "jobdispatch-smoke.exe"

try {
    # Build the same executable a local presenter can run.
    go build -o $executable .\cmd\jobdispatch

    # Start the server as a child process and retain its PID for targeted cleanup.
    $process = Start-Process -FilePath $executable -PassThru -WindowStyle Hidden

    try {
        # Poll readiness because process startup is asynchronous.
        $ready = $null
        for ($attempt = 0; $attempt -lt 30 -and $null -eq $ready; $attempt++) {
            Start-Sleep -Milliseconds 100
            try {
                $ready = Invoke-RestMethod "http://127.0.0.1:18080/health/ready"
            }
            catch {
                # The listener is not ready yet; the bounded loop will retry.
            }
        }

        if ($null -eq $ready) {
            throw "server did not become ready within three seconds"
        }

        # Exercise the same lifecycle demonstrated by the .NET reference.
        $created = Invoke-RestMethod `
            -Method Post `
            -Uri "http://127.0.0.1:18080/jobs" `
            -ContentType "application/json" `
            -Body '{"jobType":"email"}'

        $claimed = Invoke-RestMethod `
            -Method Post `
            -Uri "http://127.0.0.1:18080/jobs/$($created.id)/claim" `
            -ContentType "application/json" `
            -Body '{"leaseOwner":"smoke-worker"}'

        $completed = Invoke-RestMethod `
            -Method Post `
            -Uri "http://127.0.0.1:18080/jobs/$($created.id)/complete" `
            -ContentType "application/json" `
            -Body '{"message":"smoke complete"}'

        if ($ready.status -ne "ready" `
            -or $created.status -ne "queued" `
            -or $claimed.status -ne "claimed" `
            -or $completed.status -ne "completed") {
            throw "unexpected response in lifecycle smoke flow"
        }

        Write-Output "Go process smoke test passed."
    }
    finally {
        # Stop only the process created by this script.
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    }
}
finally {
    Remove-Item $executable -Force -ErrorAction SilentlyContinue
}
