package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
)

type JobRepository interface {
	Add(context.Context, domain.Job) (domain.Job, error)
	GetByID(context.Context, string) (domain.Job, bool, error)
	Update(context.Context, domain.Job) (domain.Job, error)
	GetEligible(context.Context, time.Time) ([]domain.Job, error)
}

type InMemoryJobRepository struct {
	mu   sync.RWMutex
	jobs map[string]domain.Job
}

func NewInMemoryJobRepository() *InMemoryJobRepository {
	return &InMemoryJobRepository{jobs: make(map[string]domain.Job)}
}

func (r *InMemoryJobRepository) Add(ctx context.Context, job domain.Job) (domain.Job, error) {
	if err := ctx.Err(); err != nil {
		return domain.Job{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return job, nil
}

func (r *InMemoryJobRepository) GetByID(ctx context.Context, id string) (domain.Job, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Job{}, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[id]
	return job, ok, nil
}

func (r *InMemoryJobRepository) Update(ctx context.Context, job domain.Job) (domain.Job, error) {
	if err := ctx.Err(); err != nil {
		return domain.Job{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return job, nil
}

func (r *InMemoryJobRepository) GetEligible(ctx context.Context, now time.Time) ([]domain.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobs := make([]domain.Job, 0)
	for _, job := range r.jobs {
		queuedDue := job.Status == domain.StatusQueued &&
			(job.NextAttemptAt == nil || !job.NextAttemptAt.After(now))
		leaseExpired := job.Status == domain.StatusClaimed &&
			(job.LeaseExpiresAt == nil || !job.LeaseExpiresAt.After(now))
		if queuedDue || leaseExpired {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})
	return jobs, nil
}
