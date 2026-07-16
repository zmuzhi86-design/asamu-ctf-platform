package challenge

import (
	"strconv"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func filter(c *gin.Context) Filter {
	page, size := httpx.PageParams(c)
	result := Filter{Search: c.Query("search"), Category: c.Query("category"), Difficulty: c.Query("difficulty"), Status: c.Query("status"), Page: page, PageSize: size}
	if value := c.Query("dynamic"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			result.Dynamic = &parsed
		}
	}
	return result
}
func (h *Handler) List(c *gin.Context) {
	page, err := h.service.List(c.Request.Context(), filter(c), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
}
func (h *Handler) AdminList(c *gin.Context) {
	page, err := h.service.List(c.Request.Context(), filter(c), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, page)
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
func (h *Handler) Publish(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.Publish(c.Request.Context(), c.Param("id"), userID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) Archive(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.Archive(c.Request.Context(), c.Param("id"), userID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
