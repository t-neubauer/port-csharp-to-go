package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/domain"
	"github.com/t-neubauer/port-csharp-to-go/GoProject/internal/service"
)

type Handler struct {
	service *service.JobService
	logger  *slog.Logger
}

func NewHandler(jobService *service.JobService, logger *slog.Logger) http.Handler {
	h := &Handler{service: jobService, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", h.createJob)
	mux.HandleFunc("GET /jobs/{id}", h.getJob)
	mux.HandleFunc("POST /jobs/{id}/claim", h.claimJob)
	mux.HandleFunc("POST /jobs/{id}/complete", h.completeJob)
	mux.HandleFunc("POST /jobs/{id}/fail", h.failJob)
	mux.HandleFunc("GET /health/live", h.live)
	mux.HandleFunc("GET /health/ready", h.ready)
	return mux
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var request domain.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body is invalid")
		return
	}
	job, err := h.service.CreateJob(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	job, err := h.service.GetJob(r.Context(), id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) claimJob(w http.ResponseWriter, r *http.Request) {
	var request domain.ClaimJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body is invalid")
		return
	}
	job, err := h.service.ClaimJob(r.Context(), r.PathValue("id"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) completeJob(w http.ResponseWriter, r *http.Request) {
	var request domain.CompleteJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body is invalid")
		return
	}
	job, err := h.service.CompleteJob(r.Context(), r.PathValue("id"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) failJob(w http.ResponseWriter, r *http.Request) {
	var request domain.FailJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "request body is invalid")
		return
	}
	job, err := h.service.FailJob(r.Context(), r.PathValue("id"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "live", "timestamp": time.Now().UTC()})
}

func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "timestamp": time.Now().UTC()})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrJobNotFound):
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, service.ErrInvalidState):
		writeError(w, http.StatusConflict, "INVALID_JOB_STATE", err.Error())
	default:
		h.logger.Error("job_request_failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an internal error occurred")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
