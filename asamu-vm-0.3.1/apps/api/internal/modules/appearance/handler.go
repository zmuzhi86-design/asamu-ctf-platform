package appearance

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) CurrentTheme(c *gin.Context) {
	theme, err := h.service.CurrentTheme(c.Request.Context(), "platform", nil)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, theme)
}
func (h *Handler) CurrentBackgrounds(c *gin.Context) {
	items, err := h.service.CurrentBackgrounds(c.Request.Context(), "platform", nil)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) ListBackgrounds(c *gin.Context) {
	items, err := h.service.ListBackgrounds(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) SaveBackground(c *gin.Context) {
	var input Background
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if raw := c.Param("id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "背景 ID 不合法"))
			return
		}
		input.ID = id
	}
	item, err := h.service.SaveBackground(c.Request.Context(), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) PublishBackground(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "背景 ID 不合法"))
		return
	}
	if err := h.service.PublishBackground(c.Request.Context(), id, actor); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) RollbackBackground(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "背景 ID 不合法"))
		return
	}
	item, err := h.service.RollbackBackground(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) SaveTheme(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Key, Name string
		Tokens    map[string]string
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	theme, err := h.service.SaveTheme(c.Request.Context(), actor, input.Key, input.Name, input.Tokens)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, theme)
}
