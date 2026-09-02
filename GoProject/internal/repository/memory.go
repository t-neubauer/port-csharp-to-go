package repository

import (
	"context"
	"sync"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
)

type JobRepository interface {
	Add(context.Context, domain.Job) (domain.Job, error)
	GetByID(context.Context, string) (domain.Job, bool, error)
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
