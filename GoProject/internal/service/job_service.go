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

type Options struct {
	DefaultMaxAttempts int
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
