package worker

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
)

func TestWorkerProcessesJobsAndStopsWhenCanceled(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	job, _ := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	ctx, cancel := context.WithCancel(context.Background())
	done := runTestWorker(ctx, svc)
	waitForStatus(t, repo, job.ID, domain.StatusCompleted)
	cancel()
	waitForWorker(t, done)
}

func TestWorkerSchedulesRetryForRetryJobs(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute, RetryBackoff: time.Hour})
	job, _ := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "retry-email"})
	ctx, cancel := context.WithCancel(context.Background())
	done := runTestWorker(ctx, svc)
	defer func() { cancel(); waitForWorker(t, done) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		processed, _, _ := repo.GetByID(context.Background(), job.ID)
		if processed.Status == domain.StatusQueued && processed.AttemptCount == 1 && processed.NextAttemptAt != nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job was not queued for retry")
}

func TestWorkerEventuallyFailsExhaustedFailJob(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{DefaultMaxAttempts: 1, LeaseDuration: time.Minute})
	job, _ := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "fail-email"})
	ctx, cancel := context.WithCancel(context.Background())
	done := runTestWorker(ctx, svc)
	waitForStatus(t, repo, job.ID, domain.StatusFailed)
	cancel()
	waitForWorker(t, done)
}

func runTestWorker(ctx context.Context, svc *service.JobService) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		New(svc, time.Millisecond, "test-worker", true, slog.Default()).Run(ctx)
		close(done)
	}()
	return done
}

func waitForWorker(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func waitForStatus(t *testing.T, repo *repository.InMemoryJobRepository, id string, want domain.JobStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		job, _, _ := repo.GetByID(context.Background(), id)
		if job.Status == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	job, _, _ := repo.GetByID(context.Background(), id)
	t.Fatalf("job status = %q, want %q", job.Status, want)
}
