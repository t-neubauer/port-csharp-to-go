package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusClaimed   JobStatus = "claimed"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

type Job struct {
	ID             string          `json:"id"`
	JobType        string          `json:"jobType"`
	Name           string          `json:"name"`
	Payload        json.RawMessage `json:"payload"`
	Status         JobStatus       `json:"status"`
	AttemptCount   int             `json:"attemptCount"`
	MaxAttempts    int             `json:"maxAttempts"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	NextAttemptAt  *time.Time      `json:"nextAttemptAt,omitempty"`
	LeaseOwner     string          `json:"leaseOwner,omitempty"`
	LeaseExpiresAt *time.Time      `json:"leaseExpiresAt,omitempty"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	FailedAt       *time.Time      `json:"failedAt,omitempty"`
	LastError      string          `json:"lastError,omitempty"`
	ErrorCode      string          `json:"errorCode,omitempty"`
}

type ClaimJobRequest struct {
	WorkerID     string `json:"workerId"`
	LeaseOwner   string `json:"leaseOwner"`
	Worker       string `json:"worker"`
	LeaseSeconds *int   `json:"leaseSeconds"`
}

type CompleteJobRequest struct {
	WorkerID string          `json:"workerId"`
	Worker   string          `json:"worker"`
	Message  string          `json:"message"`
	Result   json.RawMessage `json:"result"`
}

type FailJobRequest struct {
	WorkerID  string `json:"workerId"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Transient *bool  `json:"transient"`
	Retryable *bool  `json:"retryable"`
}

type CreateJobRequest struct {
	Type        string          `json:"type"`
	JobType     string          `json:"jobType"`
	Name        string          `json:"name"`
	Payload     json.RawMessage `json:"payload"`
	Data        json.RawMessage `json:"data"`
	Body        json.RawMessage `json:"body"`
	MaxAttempts *int            `json:"maxAttempts"`
}

func NewID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate job id: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:6]),
		hex.EncodeToString(bytes[6:8]),
		hex.EncodeToString(bytes[8:10]),
		hex.EncodeToString(bytes[10:16])), nil
}
