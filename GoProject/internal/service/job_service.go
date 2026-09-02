package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
)

var ErrJobNotFound = errors.New("job not found")
var ErrValidation = errors.New("validation error")
var ErrInvalidState = errors.New("invalid job state")

func IsExpectedJobError(err error) bool {
	return errors.Is(err, ErrJobNotFound) ||
		errors.Is(err, ErrValidation) ||
		errors.Is(err, ErrInvalidState)
}

type Options struct {
	DefaultMaxAttempts int
	LeaseDuration      time.Duration
	RetryBackoff       time.Duration
}

type JobService struct {
	repository repository.JobRepository
	options    Options
}

func NewJobService(repo repository.JobRepository, options Options) *JobService {
	return &JobService{repository: repo, options: options}
}

func (s *JobService) CreateJob(ctx context.Context, request domain.CreateJobRequest) (domain.Job, error) {
	jobType := strings.TrimSpace(request.Type)
	if jobType == "" {
		jobType = strings.TrimSpace(request.JobType)
	}
	if jobType == "" {
		jobType = strings.TrimSpace(request.Name)
	}
	if jobType == "" {
		return domain.Job{}, fmt.Errorf("%w: job type is required", ErrValidation)
	}

	maxAttempts := s.options.DefaultMaxAttempts
	if request.MaxAttempts != nil {
		maxAttempts = *request.MaxAttempts
	}
	if maxAttempts <= 0 {
		return domain.Job{}, fmt.Errorf("%w: max_attempts must be greater than zero", ErrValidation)
	}

	payload := request.Payload
	if len(payload) == 0 {
		payload = request.Data
	}
	if len(payload) == 0 {
		payload = request.Body
	}
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	now := time.Now().UTC()
	id, err := domain.NewID()
	if err != nil {
		return domain.Job{}, err
	}
	job := domain.Job{
		ID:            id,
		JobType:       jobType,
		Name:          request.Name,
		Payload:       payload,
		Status:        domain.StatusQueued,
		MaxAttempts:   maxAttempts,
		CreatedAt:     now,
		UpdatedAt:     now,
		NextAttemptAt: &now,
	}
	if job.Name == "" {
		job.Name = jobType
	}
	return s.repository.Add(ctx, job)
}

func (s *JobService) GetJob(ctx context.Context, id string) (domain.Job, error) {
	job, ok, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if !ok {
		return domain.Job{}, fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}
	return job, nil
}

func (s *JobService) ClaimJob(ctx context.Context, id string, request domain.ClaimJobRequest) (domain.Job, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	now := time.Now().UTC()
	if job.Status == domain.StatusCompleted || job.Status == domain.StatusFailed {
		return domain.Job{}, fmt.Errorf("%w: job is already terminal", ErrInvalidState)
	}
	if job.Status == domain.StatusClaimed && job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now) {
		return domain.Job{}, fmt.Errorf("%w: job is already claimed", ErrInvalidState)
	}
	if job.Status == domain.StatusQueued && job.NextAttemptAt != nil && job.NextAttemptAt.After(now) {
		return domain.Job{}, fmt.Errorf("%w: retry is not due", ErrInvalidState)
	}
	leaseDuration := s.options.LeaseDuration
	if request.LeaseSeconds != nil {
		leaseDuration = time.Duration(*request.LeaseSeconds) * time.Second
	}
	if leaseDuration <= 0 {
		return domain.Job{}, fmt.Errorf("%w: lease_seconds must be greater than zero", ErrValidation)
	}
	owner := strings.TrimSpace(request.LeaseOwner)
	if owner == "" {
		owner = strings.TrimSpace(request.WorkerID)
	}
	if owner == "" {
		owner = strings.TrimSpace(request.Worker)
	}
	if owner == "" {
		owner = "worker"
	}
	expires := now.Add(leaseDuration)
	job.Status = domain.StatusClaimed
	job.AttemptCount++
	job.LeaseOwner = owner
	job.LeaseExpiresAt = &expires
	job.NextAttemptAt = nil
	job.UpdatedAt = now
	return s.repository.Update(ctx, job)
}

func (s *JobService) CompleteJob(ctx context.Context, id string, request domain.CompleteJobRequest) (domain.Job, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if job.Status == domain.StatusCompleted {
		return job, nil
	}
	if job.Status != domain.StatusClaimed {
		return domain.Job{}, fmt.Errorf("%w: job must be claimed before completion", ErrInvalidState)
	}
	now := time.Now().UTC()
	job.Status = domain.StatusCompleted
	job.CompletedAt = &now
	job.UpdatedAt = now
	job.LeaseOwner = ""
	job.LeaseExpiresAt = nil
	job.NextAttemptAt = nil
	job.LastError = request.Message
	return s.repository.Update(ctx, job)
}

func (s *JobService) FailJob(ctx context.Context, id string, request domain.FailJobRequest) (domain.Job, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if job.Status == domain.StatusCompleted || job.Status == domain.StatusFailed {
		return job, nil
	}
	if job.Status != domain.StatusClaimed {
		return domain.Job{}, fmt.Errorf("%w: job must be claimed before failure", ErrInvalidState)
	}
	now := time.Now().UTC()
	transient := request.Transient != nil && *request.Transient
	if request.Retryable != nil && *request.Retryable {
		transient = true
	}
	if transient && job.AttemptCount < job.MaxAttempts {
		next := now.Add(s.options.RetryBackoff)
		job.Status = domain.StatusQueued
		job.NextAttemptAt = &next
		job.LeaseOwner = ""
		job.LeaseExpiresAt = nil
		job.LastError = failureMessage(request)
		job.UpdatedAt = now
		return s.repository.Update(ctx, job)
	}
	job.Status = domain.StatusFailed
	job.FailedAt = &now
	job.NextAttemptAt = nil
	job.LeaseOwner = ""
	job.LeaseExpiresAt = nil
	job.LastError = failureMessage(request)
	job.ErrorCode = "JOB_FAILED"
	job.UpdatedAt = now
	return s.repository.Update(ctx, job)
}

func (s *JobService) GetEligibleJobs(ctx context.Context) ([]domain.Job, error) {
	return s.repository.GetEligible(ctx, time.Now().UTC())
}

func (s *JobService) ProcessQueuedJob(ctx context.Context, id string, workerName string) (domain.Job, error) {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	if job.Status == domain.StatusCompleted || job.Status == domain.StatusFailed {
		return job, nil
	}
	if job.Status == domain.StatusQueued && job.NextAttemptAt != nil && job.NextAttemptAt.After(time.Now().UTC()) {
		return job, nil
	}

	claimed, err := s.ClaimJob(ctx, id, domain.ClaimJobRequest{
		LeaseOwner: workerName,
	})
	if err != nil {
		return domain.Job{}, err
	}

	switch {
	case strings.Contains(claimed.JobType, "fail") || strings.Contains(claimed.Name, "fail"):
		transient := true
		return s.FailJob(ctx, id, domain.FailJobRequest{
			Error:     "worker failure simulated.",
			Transient: &transient,
		})
	case strings.Contains(claimed.JobType, "retry") || strings.Contains(claimed.Name, "retry"):
		transient := true
		return s.FailJob(ctx, id, domain.FailJobRequest{
			Error:     "transient retry required.",
			Transient: &transient,
		})
	default:
		return s.CompleteJob(ctx, id, domain.CompleteJobRequest{
			Message: "processed by worker",
		})
	}
}

func failureMessage(request domain.FailJobRequest) string {
	if strings.TrimSpace(request.Error) != "" {
		return request.Error
	}
	if strings.TrimSpace(request.Message) != "" {
		return request.Message
	}
	return "job processing failed"
}
