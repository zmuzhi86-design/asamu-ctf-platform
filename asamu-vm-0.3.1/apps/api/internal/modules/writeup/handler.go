package writeup

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), c.Query("category"), c.Query("search"), page, size, false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) AdminList(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), c.Query("category"), c.Query("search"), page, size, true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Mine(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, size := httpx.PageParams(c)
	items, err := h.service.Mine(c.Request.Context(), userID, page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) MineDetail(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.MineDetail(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Detail(c *gin.Context) {
	var viewer *uuid.UUID
	if id, err := httpx.UserID(c); err == nil {
		viewer = &id
	}
	item, err := h.service.Detail(c.Request.Context(), c.Param("id"), viewer, false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Create(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input Mutation
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Update(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input Mutation
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), userID, c.Param("id"), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) SubmitReview(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.SubmitReview(c.Request.Context(), userID, c.Param("id")); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Review(c *gin.Context) {
	reviewer, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct{ Action, Note string }
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.Review(c.Request.Context(), reviewer, c.Param("id"), input.Action, input.Note); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Comment(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	comment, err := h.service.Comment(c.Request.Context(), userID, c.Param("id"), input.Content)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, comment)
}
func (h *Handler) Like(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	liked, err := h.service.ToggleLike(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, map[string]bool{"liked": liked})
}
func (h *Handler) Favorite(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.Favorite(c.Request.Context(), userID, c.Param("id"), input.Enabled); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, map[string]bool{"favorited": input.Enabled})
}
