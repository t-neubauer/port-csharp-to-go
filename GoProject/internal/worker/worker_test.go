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
	svc := service.NewJobService(repo, service.Options{
		DefaultMaxAttempts: 3,
		LeaseDuration:      time.Minute,
		RetryBackoff:       time.Millisecond,
	})
	job, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		New(svc, time.Millisecond, "test-worker", true, slog.Default()).Run(ctx)
		close(done)
	}()

	waitForStatus(t, repo, job.ID, domain.StatusCompleted)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestWorkerSchedulesRetryForRetryJobs(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{
		DefaultMaxAttempts: 3,
		LeaseDuration:      time.Minute,
		RetryBackoff:       time.Hour,
	})
	job, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "retry-email"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		New(svc, time.Millisecond, "test-worker", true, slog.Default()).Run(ctx)
		close(done)
	}()

	var processed domain.Job
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		processed, _, _ = repo.GetByID(context.Background(), job.ID)
		if processed.Status == domain.StatusQueued && processed.AttemptCount == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	if processed.Status != domain.StatusQueued || processed.NextAttemptAt == nil {
		t.Fatalf("job was not queued for retry: %+v", processed)
	}
	cancel()
	<-done
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
