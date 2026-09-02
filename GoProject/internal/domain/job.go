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
	StatusQueued JobStatus = "queued"
)

type Job struct {
	ID            string          `json:"id"`
	JobType       string          `json:"jobType"`
	Name          string          `json:"name"`
	Payload       json.RawMessage `json:"payload"`
	Status        JobStatus       `json:"status"`
	AttemptCount  int             `json:"attemptCount"`
	MaxAttempts   int             `json:"maxAttempts"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	NextAttemptAt *time.Time      `json:"nextAttemptAt,omitempty"`
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
