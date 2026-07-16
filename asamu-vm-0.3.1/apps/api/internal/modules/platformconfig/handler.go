package platformconfig

import (
	"net/http"
	"regexp"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var directionSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Bootstrap(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	value, err := h.service.Bootstrap(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, value)
}
func (h *Handler) Draft(c *gin.Context) {
	value, err := h.service.Draft(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, value)
}
func (h *Handler) SaveDraft(c *gin.Context) {
	var input Bootstrap
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	value, err := h.service.SaveDraft(c.Request.Context(), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, value)
}
func (h *Handler) Publish(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	value, err := h.service.Publish(c.Request.Context(), actor)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, value)
}
func (h *Handler) Directions(c *gin.Context) {
	rows, err := h.service.Directions(c.Request.Context(), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}
func (h *Handler) AdminDirections(c *gin.Context) {
	rows, err := h.service.Directions(c.Request.Context(), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}
func (h *Handler) SaveDirection(c *gin.Context) {
	var input Direction
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if raw := c.Param("id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "方向 ID 不合法"))
			return
		}
		input.ID = id
	}
	if !directionSlugPattern.MatchString(input.Slug) {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_SLUG", "方向 Slug 只能包含小写字母、数字和连字符"))
		return
	}
	if input.Name == "" {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "NAME_REQUIRED", "方向名称不能为空"))
		return
	}
	if input.Status != "" && input.Status != "active" && input.Status != "disabled" && input.Status != "archived" {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_STATUS", "方向状态不合法"))
		return
	}
	value, err := h.service.SaveDirection(c.Request.Context(), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, value)
}
func (h *Handler) ArchiveDirection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "方向 ID 不合法"))
		return
	}
	if err := h.service.ArchiveDirection(c.Request.Context(), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
