package httpapi

import (
	"net/http"
	"strings"

	"parser-svc/internal/domain"
	"parser-svc/internal/service/desiredstate"

	"github.com/gin-gonic/gin"
)

type desiredStateReader interface {
	GetSnapshot() domain.Snapshot
	ListHosts(query desiredstate.Query) []domain.DesiredHost
	GetHostByID(hostID string) (domain.DesiredHost, bool)
	IsReady() bool
}

type Handler struct {
	service desiredStateReader
}

func NewHandler(service desiredStateReader) *Handler {
	return &Handler{service: service}
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
// @Description Returns readiness state based on startup manifest loading.
// @Tags health
// @Produce json
// @Success 200 {object} ReadinessResponse
// @Failure 503 {object} ReadinessResponse
// @Router /readyz [get]
func (h *Handler) Readyz(ctx *gin.Context) {
	snapshot := h.service.GetSnapshot()
	metadata := snapshot.Metadata

	payload := ReadinessResponse{
		Status:      "ready",
		ReadyReason: metadata.ReadyReason,
		LoadedFiles: metadata.FilesLoaded,
		BrokenFiles: metadata.FilesBroken,
		HostsTotal:  metadata.HostsTotal,
	}

	if !h.service.IsReady() {
		payload.Status = "not_ready"
		ctx.JSON(http.StatusServiceUnavailable, payload)
		return
	}

	ctx.JSON(http.StatusOK, payload)
}

// GetDesiredState godoc
// @Summary Get full desired state
// @Description Returns full in-memory desired state with metadata.
// @Tags desired-state
// @Produce json
// @Success 200 {object} DesiredStateResponse
// @Router /api/v1/desired-state [get]
func (h *Handler) GetDesiredState(ctx *gin.Context) {
	snapshot := h.service.GetSnapshot()
	ctx.JSON(http.StatusOK, toDesiredStateResponse(snapshot))
}

// ListHosts godoc
// @Summary List desired hosts
// @Description Returns hosts from desired state. Supports fqdn query and label-based filters with AND semantics.
// @Tags desired-state
// @Produce json
// @Param fqdn query string false "Host FQDN filter"
// @Param env query string false "Label filter example"
// @Param managed_by query string false "Label filter example"
// @Param purpose query string false "Label filter example"
// @Success 200 {object} HostsResponse
// @Router /api/v1/desired-state/hosts [get]
func (h *Handler) ListHosts(ctx *gin.Context) {
	query := parseHostQuery(ctx)
	hosts := h.service.ListHosts(query)
	snapshot := h.service.GetSnapshot()

	hostDTOs := make([]DesiredHostDTO, 0, len(hosts))
	for _, host := range hosts {
		hostDTOs = append(hostDTOs, toDesiredHostDTO(host))
	}

	ctx.JSON(http.StatusOK, HostsResponse{
		Metadata: HostsResponseMetadata{
			LoadedAt:       snapshot.Metadata.LoadedAt,
			TotalHosts:     snapshot.Metadata.HostsTotal,
			ReturnedHosts:  len(hostDTOs),
			WorkloadsTotal: snapshot.Metadata.WorkloadsTotal,
			FilesLoaded:    snapshot.Metadata.FilesLoaded,
			FilesBroken:    snapshot.Metadata.FilesBroken,
		},
		Hosts: hostDTOs,
	})
}

// GetHostByID godoc
// @Summary Get desired host by ID
// @Description Returns full desired host details.
// @Tags desired-state
// @Produce json
// @Param hostId path string true "Host ID"
// @Success 200 {object} HostResponse
// @Failure 404 {object} APIError
// @Router /api/v1/desired-state/hosts/{hostId} [get]
func (h *Handler) GetHostByID(ctx *gin.Context) {
	hostID := strings.TrimSpace(ctx.Param("hostId"))
	host, found := h.service.GetHostByID(hostID)
	if !found {
		ctx.JSON(http.StatusNotFound, APIError{Message: "host not found"})
		return
	}

	snapshot := h.service.GetSnapshot()
	ctx.JSON(http.StatusOK, HostResponse{
		Metadata: HostResponseMetadata{
			LoadedAt:    snapshot.Metadata.LoadedAt,
			FilesLoaded: snapshot.Metadata.FilesLoaded,
			FilesBroken: snapshot.Metadata.FilesBroken,
		},
		Host: toDesiredHostDTO(host),
	})
}

func parseHostQuery(ctx *gin.Context) desiredstate.Query {
	filters := make(map[string]string)
	fqdn := ""

	for key, values := range ctx.Request.URL.Query() {
		if len(values) == 0 {
			continue
		}

		value := strings.TrimSpace(values[0])
		if value == "" {
			continue
		}

		if key == "fqdn" {
			fqdn = value
			continue
		}

		filters[key] = value
	}

	return desiredstate.Query{
		FQDN:         fqdn,
		LabelFilters: filters,
	}
}
