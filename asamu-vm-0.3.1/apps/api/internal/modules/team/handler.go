package team

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	assetmodule "asamu.local/platform/api/internal/modules/asset"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

const maxAvatarUploadSize int64 = 5 << 20

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	page, size := httpx.PageParams(c)
	var recruiting *bool
	if raw := c.Query("recruiting"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err == nil {
			recruiting = &value
		}
	}
	items, err := h.service.List(c.Request.Context(), c.Query("search"), recruiting, page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Detail(c *gin.Context) {
	item, err := h.service.Detail(c.Request.Context(), c.Param("id"))
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
	var input CreateInput
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
func (h *Handler) RequestJoin(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Message string `json:"message"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.RequestJoin(c.Request.Context(), userID, c.Param("id"), input.Message); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) ReviewJoin(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	requestID, err := uuid.Parse(c.Param("requestId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "申请 ID 不合法"))
		return
	}
	var input struct {
		Approve bool `json:"approve"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ReviewJoin(c.Request.Context(), actor, requestID, input.Approve); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Invite(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Username string `json:"username"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := h.service.Invite(c.Request.Context(), actor, c.Param("id"), input.Username)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, map[string]any{"id": id})
}
func (h *Handler) AcceptInvitation(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("invitationId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "邀请 ID 不合法"))
		return
	}
	if err := h.service.AcceptInvitation(c.Request.Context(), userID, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Manage(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Manage(c.Request.Context(), userID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Update(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input UpdateInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), actor, c.Param("id"), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) UploadAvatar(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarUploadSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(c, httpx.NewError(http.StatusRequestEntityTooLarge, "TEAM_AVATAR_TOO_LARGE", "战队头像不能超过 5 MB"))
			return
		}
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "FILE_REQUIRED", "请选择战队头像"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAvatarUploadSize+1))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.UploadAvatar(c.Request.Context(), actor, c.Param("id"), assetmodule.Upload{Name: header.Filename, ContentType: header.Header.Get("Content-Type"), Data: data})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) TransferCaptain(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "战队 ID 无效"))
		return
	}
	var input struct {
		UserID uuid.UUID `json:"userId" binding:"required"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.TransferCaptain(c.Request.Context(), actor, teamID, input.UserID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) RemoveMember(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "战队 ID 无效"))
		return
	}
	targetID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "用户 ID 无效"))
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), actor, teamID, targetID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Leave(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "战队 ID 无效"))
		return
	}
	if err := h.service.Leave(c.Request.Context(), userID, teamID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) PostAnnouncement(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "战队 ID 无效"))
		return
	}
	var input struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
		Pinned  bool   `json:"pinned"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if len(input.Title) > 160 || len(input.Content) > 5000 {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ANNOUNCEMENT", "公告标题或正文过长"))
		return
	}
	if err := h.service.PostAnnouncement(c.Request.Context(), actor, teamID, input.Title, input.Content, input.Pinned); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
