package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
)

func TestCreateAndGetJob(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := NewJobService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})

	created, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{
		JobType: "email",
		Payload: []byte(`{"to":"ops@example.com"}`),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if created.Status != domain.StatusQueued || created.AttemptCount != 0 {
		t.Fatalf("unexpected created job: %+v", created)
	}

	read, err := svc.GetJob(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if read.ID != created.ID || read.JobType != "email" {
		t.Fatalf("unexpected read job: %+v", read)
	}
}

func TestClaimCompleteAndRetry(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := NewJobService(repo, Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	job, err := svc.CreateJob(context.Background(), domain.CreateJobRequest{JobType: "email"})
	if err != nil {
		t.Fatal(err)
	}

	claimed, err := svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "worker-a"})
	if err != nil || claimed.Status != domain.StatusClaimed || claimed.AttemptCount != 1 {
		t.Fatalf("claim = %+v, err = %v", claimed, err)
	}
	if _, err = svc.ClaimJob(context.Background(), job.ID, domain.ClaimJobRequest{LeaseOwner: "worker-b"}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second claim error = %v, want invalid state", err)
	}

	transient := true
	retried, err := svc.FailJob(context.Background(), job.ID, domain.FailJobRequest{Error: "temporary outage", Transient: &transient})
	if err != nil || retried.Status != domain.StatusQueued || retried.NextAttemptAt == nil {
		t.Fatalf("retry = %+v, err = %v", retried, err)
	}

	time.Sleep(10 * time.Millisecond)
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
