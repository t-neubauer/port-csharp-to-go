package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
)

func newService(repo *repository.InMemoryJobRepository, options Options) *JobService {
	return NewJobService(repo, options)
}

func TestCreateAndGetJob(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := newService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	created, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email", Payload: []byte(`{"to":"ops@example.com"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != domain.StatusQueued || created.AttemptCount != 0 {
		t.Fatalf("unexpected created job: %+v", created)
	}
	read, err := svc.GetJob(context.Background(), created.ID)
	if err != nil || read.ID != created.ID || read.JobType != "email" {
		t.Fatalf("unexpected read job: %+v, err=%v", read, err)
	}
}

func TestClaimCompleteAndRetry(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := newService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	job, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "worker-a"})
	if err != nil || claimed.Status != domain.StatusClaimed || claimed.AttemptCount != 1 {
		t.Fatalf("claim = %+v, err = %v", claimed, err)
	}
	if _, err = svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "worker-b"}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second claim error = %v", err)
	}
	transient := true
	retried, err := svc.FailJob(context.Background(), job.ID, domain.FailJobRequest{Error: "temporary outage", Transient: &transient})
	if err != nil || retried.Status != domain.StatusQueued || retried.NextAttemptAt == nil {
		t.Fatalf("retry = %+v, err = %v", retried, err)
	}
	retried.NextAttemptAt = timePtr(time.Now().UTC().Add(-time.Second))
	if _, err = repo.Update(context.Background(), retried); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "worker-c"})
	if err != nil || reclaimed.Status != domain.StatusClaimed {
		t.Fatalf("reclaim = %+v, err = %v", reclaimed, err)
	}
	completed, err := svc.CompleteJob(context.Background(), job.ID, domain.CompleteJobRequest{Message: "done"})
	if err != nil || completed.Status != domain.StatusCompleted {
		t.Fatalf("complete = %+v, err = %v", completed, err)
	}
	repeated, err := svc.CompleteJob(context.Background(), job.ID, domain.CompleteJobRequest{})
	if err != nil || repeated.AttemptCount != completed.AttemptCount {
		t.Fatalf("repeated complete = %+v, err = %v", repeated, err)
	}
}

func TestClaimRejectsRetryBeforeDueTime(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := newService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	job, _ := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	future := time.Now().UTC().Add(time.Hour)
	job.NextAttemptAt = &future
	_, _ = repo.Update(context.Background(), job)
	if _, err := svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("claim error = %v", err)
	}
}

func TestExpiredLeaseCanBeReclaimed(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := newService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	expired := time.Now().UTC().Add(-time.Second)
	job := domain.Job{ID: "expired-job", JobType: "email", Name: "email", Status: domain.StatusClaimed, AttemptCount: 1, MaxAttempts: 3, LeaseExpiresAt: &expired}
	_, _ = repo.Add(context.Background(), job)
	reclaimed, err := svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "new-worker"})
	if err != nil || reclaimed.LeaseOwner != "new-worker" || reclaimed.AttemptCount != 2 {
		t.Fatalf("unexpected reclaimed job: %+v, err=%v", reclaimed, err)
	}
}

func TestNonTransientFailureIsTerminalAndIdempotent(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := newService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	job, _ := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	_, _ = svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{})
	failed, err := svc.FailJob(context.Background(), job.ID, domain.FailJobRequest{Error: "permanent"})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := svc.FailJob(context.Background(), job.ID, domain.FailJobRequest{Error: "duplicate"})
	if err != nil || failed.Status != domain.StatusFailed || repeated.Status != domain.StatusFailed ||
		repeated.AttemptCount != failed.AttemptCount || repeated.ErrorCode != "JOB_FAILED" {
		t.Fatalf("failed=%+v repeated=%+v err=%v", failed, repeated, err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
