package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
)

func TestCreateAndGetJobContract(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	handler := NewHandler(svc, slog.Default())

	create := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(`{"jobType":"email","payload":{"to":"ops@example.com"}}`))
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, create)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRecorder.Code, http.StatusCreated)
	}

	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(createRecorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if response.ID == "" || response.Status != "queued" {
		t.Fatalf("unexpected create response: %+v", response)
	}

	get := httptest.NewRequest(http.MethodGet, "/jobs/"+response.ID, nil)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}
}

func TestClaimCompleteAndFailContract(t *testing.T) {
	repo := repository.NewInMemoryJobRepository()
	svc := service.NewJobService(repo, service.Options{DefaultMaxAttempts: 3, LeaseDuration: time.Minute})
	handler := NewHandler(svc, slog.Default())

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(`{"jobType":"email"}`)))
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	claim := httptest.NewRecorder()
	handler.ServeHTTP(claim, httptest.NewRequest(http.MethodPost, "/jobs/"+created.ID+"/claim", bytes.NewBufferString(`{"leaseOwner":"worker-a"}`)))
	if claim.Code != http.StatusOK {
		t.Fatalf("claim status = %d", claim.Code)
	}

	secondClaim := httptest.NewRecorder()
	handler.ServeHTTP(secondClaim, httptest.NewRequest(http.MethodPost, "/jobs/"+created.ID+"/claim", bytes.NewBufferString(`{"leaseOwner":"worker-b"}`)))
	if secondClaim.Code != http.StatusConflict {
		t.Fatalf("second claim status = %d, want %d", secondClaim.Code, http.StatusConflict)
	}

	complete := httptest.NewRecorder()
	handler.ServeHTTP(complete, httptest.NewRequest(http.MethodPost, "/jobs/"+created.ID+"/complete", bytes.NewBufferString(`{"message":"done"}`)))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete status = %d", complete.Code)
	}
}
