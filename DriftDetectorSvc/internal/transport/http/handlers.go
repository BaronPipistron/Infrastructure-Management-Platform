package httpapi

import (
	"context"
	"errors"
	"net/http"

	"drift-detector-svc/internal/domain"
	"drift-detector-svc/internal/service/detection"

	"github.com/gin-gonic/gin"
)

type detectionRunner interface {
	RunCycle(ctx context.Context, trigger string) (domain.DetectionCycleResult, error)
	IsReady() bool
	LastCycleResult() (domain.DetectionCycleResult, bool)
}

type Handler struct {
	detectionService detectionRunner
}

func NewHandler(detectionService detectionRunner) *Handler {
	return &Handler{detectionService: detectionService}
}

// Healthz godoc
// @Summary Health check
// @Description Returns service liveness state.
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /healthz [get]
func (h *Handler) Healthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// Readyz godoc
// @Summary Readiness check
// @Description Returns readiness state based on successful detection cycle execution.
// @Tags health
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /readyz [get]
func (h *Handler) Readyz(ctx *gin.Context) {
	lastResult, hasResult := h.detectionService.LastCycleResult()

	response := ReadinessResponse{Status: "ready"}
	if hasResult {
		response.LastCycleID = lastResult.CycleID
		finishedAt := lastResult.FinishedAt.UTC()
		response.LastCycleAt = &finishedAt
		response.LastCyclePartial = lastResult.Partial
	}

	if !h.detectionService.IsReady() {
		response.Status = "not_ready"
		ctx.JSON(http.StatusServiceUnavailable, response)
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// RunDetectionCycle godoc
// @Summary Run one detection cycle manually
// @Description Triggers one immediate drift detection cycle. Scheduler keeps running independently.
// @Tags detection
// @Produce json
// @Success 200 {object} DetectionRunResponse
// @Failure 409 {object} APIError
// @Failure 502 {object} APIError
// @Router /api/v1/detection/run [post]
func (h *Handler) RunDetectionCycle(ctx *gin.Context) {
	result, err := h.detectionService.RunCycle(ctx.Request.Context(), "manual_api")
	if err != nil {
		if errors.Is(err, detection.ErrCycleAlreadyRunning) {
			ctx.JSON(http.StatusConflict, APIError{Message: "detection cycle already running"})
			return
		}

		ctx.JSON(http.StatusBadGateway, APIError{Message: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, DetectionRunResponse{
		Status: "completed",
		Result: toDetectionCycleDTO(result),
	})
}
