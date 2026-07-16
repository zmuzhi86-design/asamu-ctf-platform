package admin

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Dashboard(c *gin.Context) {
	data, err := h.service.Dashboard(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, data)
}
func (h *Handler) Users(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.Users(c.Request.Context(), c.Query("search"), c.Query("status"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) SetUserStatus(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "用户 ID 不合法"))
		return
	}
	var input struct{ Status, Reason string }
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.SetUserStatus(c.Request.Context(), actor, id, input.Status, input.Reason); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) AssignRole(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "用户 ID 不合法"))
		return
	}
	var input struct {
		Role    string
		Enabled bool
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.AssignRole(c.Request.Context(), actor, id, input.Role, input.Enabled); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Submissions(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.Submissions(c.Request.Context(), c.Query("result"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) CheatCases(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.CheatCases(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) ResolveCheatCase(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "案件 ID 不合法"))
		return
	}
	var input struct{ Status, Resolution, Note string }
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ResolveCheatCase(c.Request.Context(), actor, id, input.Status, input.Resolution, input.Note); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Audit(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.Audit(c.Request.Context(), c.Query("resourceType"), c.Query("actor"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Announcements(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.Announcements(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) CreateAnnouncement(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	input := AnnouncementInput{}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.CreateAnnouncement(c.Request.Context(), actor, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
