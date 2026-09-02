package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/repository"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
)

func newTestHandler(maxAttempts int) http.Handler {
	return NewHandler(service.NewJobService(
		repository.NewInMemoryJobRepository(),
		service.Options{DefaultMaxAttempts: maxAttempts, LeaseDuration: time.Minute},
	), slog.Default())
}

func TestCreateAndGetJobContract(t *testing.T) {
	handler := newTestHandler(3)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(`{"jobType":"email","payload":{"to":"ops@example.com"}}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusCreated)
	}

	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(create.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.ID == "" || response.Status != "queued" {
		t.Fatalf("unexpected create response: %+v", response)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/jobs/"+response.ID, nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", get.Code, http.StatusOK)
	}
}

func TestClaimCompleteAndFailContract(t *testing.T) {
	handler := newTestHandler(3)
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

func TestHealthContractIncludesStatusAndTimestamp(t *testing.T) {
	handler := newTestHandler(3)
	for _, route := range []string{"/health/live", "/health/ready"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", route, recorder.Code)
		}
		var response struct {
			Status    string    `json:"status"`
			Timestamp time.Time `json:"timestamp"`
		}
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.Status == "" || response.Timestamp.IsZero() {
			t.Fatalf("%s response missing fields: %+v", route, response)
		}
	}
}

func TestFailContractSupportsTerminalIdempotency(t *testing.T) {
	handler := newTestHandler(1)
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

	fail := httptest.NewRecorder()
	handler.ServeHTTP(fail, httptest.NewRequest(http.MethodPost, "/jobs/"+created.ID+"/fail", bytes.NewBufferString(`{"error":"terminal failure","transient":true}`)))
	if fail.Code != http.StatusOK || !strings.Contains(fail.Body.String(), `"status":"failed"`) {
		t.Fatalf("fail response = %d %s", fail.Code, fail.Body.String())
	}
	repeat := httptest.NewRecorder()
	handler.ServeHTTP(repeat, httptest.NewRequest(http.MethodPost, "/jobs/"+created.ID+"/fail", bytes.NewBufferString(`{"error":"duplicate","transient":true}`)))
	if repeat.Code != http.StatusOK {
		t.Fatalf("repeat fail status = %d", repeat.Code)
	}
}

func TestValidationAndNotFoundErrorsUseFrozenCodes(t *testing.T) {
	handler := newTestHandler(3)
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewBufferString(`{}`)))
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"VALIDATION_ERROR"`) {
		t.Fatalf("invalid response = %d %s", invalid.Code, invalid.Body.String())
	}
	notFound := httptest.NewRecorder()
	handler.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "/jobs/does-not-exist", nil))
	if notFound.Code != http.StatusNotFound || !strings.Contains(notFound.Body.String(), `"code":"JOB_NOT_FOUND"`) {
		t.Fatalf("not found response = %d %s", notFound.Code, notFound.Body.String())
	}
}
