package httpapi

import (
	"net/http"
	"strings"

	"inventory-svc/internal/domain"

	"github.com/gin-gonic/gin"
)

type inventoryReader interface {
	ListHosts(labelFilters map[string]string) ([]domain.Host, domain.InventoryMetadata)
	GetHostByID(id string) (domain.Host, bool)
	IsReady() bool
	GetMetadata() domain.InventoryMetadata
}

type Handler struct {
	inventory inventoryReader
}

func NewHandler(inventory inventoryReader) *Handler {
	return &Handler{inventory: inventory}
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
// @Description Returns readiness state after initial inventory sync.
// @Tags health
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /readyz [get]
func (h *Handler) Readyz(ctx *gin.Context) {
	metadata := h.inventory.GetMetadata()
	if !h.inventory.IsReady() {
		ctx.JSON(http.StatusServiceUnavailable, ReadinessResponse{
			Status:     "not_ready",
			IsPartial:  metadata.IsPartial,
			LastSyncAt: metadata.LastSyncAt,
		})
		return
	}

	ctx.JSON(http.StatusOK, ReadinessResponse{
		Status:     "ready",
		IsPartial:  metadata.IsPartial,
		LastSyncAt: metadata.LastSyncAt,
	})
}

// ListHosts godoc
// @Summary List hosts
// @Description Returns inventory hosts with optional label filtering. Query params are interpreted as label filters with AND semantics.
// @Tags hosts
// @Produce json
// @Param managed_by query string false "Label filter example"
// @Param env query string false "Label filter example"
// @Param purpose query string false "Label filter example"
// @Success 200 {object} HostsResponse
// @Router /api/v1/hosts [get]
func (h *Handler) ListHosts(ctx *gin.Context) {
	filters := parseLabelFilters(ctx)
	hosts, metadata := h.inventory.ListHosts(filters)

	hostDTOs := make([]HostDTO, 0, len(hosts))
	for _, host := range hosts {
		hostDTOs = append(hostDTOs, toHostDTO(host))
	}

	ctx.JSON(http.StatusOK, HostsResponse{
		Metadata: toHostsResponseMetadata(metadata, len(hostDTOs)),
		Hosts:    hostDTOs,
	})
}

// GetHost godoc
// @Summary Get host by ID
// @Description Returns full host details including workloads and per-source statuses.
// @Tags hosts
// @Produce json
// @Param id path string true "Host ID"
// @Success 200 {object} HostResponse
// @Failure 404 {object} APIError
// @Router /api/v1/hosts/{id} [get]
func (h *Handler) GetHost(ctx *gin.Context) {
	hostID := strings.TrimSpace(ctx.Param("id"))
	host, found := h.inventory.GetHostByID(hostID)
	if !found {
		ctx.JSON(http.StatusNotFound, APIError{Message: "host not found"})
		return
	}

	metadata := h.inventory.GetMetadata()
	ctx.JSON(http.StatusOK, HostResponse{
		Metadata: HostResponseMetadata{
			IsPartial:  metadata.IsPartial,
			LastSyncAt: metadata.LastSyncAt,
		},
		Host: toHostDTO(host),
	})
}

func parseLabelFilters(ctx *gin.Context) map[string]string {
	query := ctx.Request.URL.Query()
	filters := make(map[string]string)

	for key, values := range query {
		if len(values) == 0 {
			continue
		}
		value := strings.TrimSpace(values[0])
		if value == "" {
			continue
		}
		filters[key] = value
	}

	return filters
}
