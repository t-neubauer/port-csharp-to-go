package service

import (
	"context"
	"testing"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
)

func TestCreateAndGetJob(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := NewJobService(repo, Options{DefaultMaxAttempts: 3})

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
