package competition

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), c.Query("status"), page, size, false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) AdminList(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), c.Query("status"), page, size, true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Detail(c *gin.Context) {
	item, err := h.service.Detail(c.Request.Context(), c.Param("id"), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) AdminDetail(c *gin.Context) {
	item, err := h.service.Detail(c.Request.Context(), c.Param("id"), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Create(c *gin.Context) {
	var input Mutation
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Update(c *gin.Context) {
	var input Mutation
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Register(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		TeamID *uuid.UUID `json:"teamId"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.Register(c.Request.Context(), userID, c.Param("id"), input.TeamID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) SetStatus(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.SetStatus(c.Request.Context(), c.Param("id"), input.Status, actor); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
