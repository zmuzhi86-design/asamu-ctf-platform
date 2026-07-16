package instance

import (
	"net/http"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service, _ *gorm.DB) *Handler { return &Handler{service: service} }
func (h *Handler) Status(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	scope, err := scopeFromQuery(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Status(c.Request.Context(), userID, c.Param("id"), scope)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Start(c *gin.Context)   { h.userOperation(c, "start") }
func (h *Handler) Restart(c *gin.Context) { h.userOperation(c, "restart") }
func (h *Handler) Stop(c *gin.Context)    { h.userOperation(c, "stop") }
func (h *Handler) Reset(c *gin.Context)   { h.userOperation(c, "reset") }
func (h *Handler) userOperation(c *gin.Context, operation string) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body struct{ CompetitionID, TeamID *uuid.UUID }
	if c.Request.ContentLength > 0 {
		if err := httpx.BindJSON(c, &body); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	scope := Scope{CompetitionID: body.CompetitionID, TeamID: body.TeamID}
	key := c.GetString("idempotency_key")
	requestID := httpx.RequestID(c)
	var item View
	switch operation {
	case "start":
		item, err = h.service.Start(c.Request.Context(), userID, c.Param("id"), key, requestID, scope)
	case "restart":
		item, err = h.service.Restart(c.Request.Context(), userID, c.Param("id"), key, requestID, scope)
	case "stop":
		item, err = h.service.Stop(c.Request.Context(), userID, c.Param("id"), key, requestID, scope)
	case "reset":
		item, err = h.service.Reset(c.Request.Context(), userID, c.Param("id"), key, requestID, scope)
	}
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, item)
}
func (h *Handler) Extend(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	scope, err := scopeFromQuery(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req struct {
		Seconds int `json:"seconds"`
	}
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Extend(c.Request.Context(), userID, c.Param("id"), req.Seconds, scope)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) AdminList(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.AdminList(c.Request.Context(), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) AdminDetail(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "实例 ID 不合法"))
		return
	}
	item, err := h.service.AdminDetail(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) AdminStop(c *gin.Context)  { h.adminOperation(c, "stop") }
func (h *Handler) AdminReset(c *gin.Context) { h.adminOperation(c, "reset") }
func (h *Handler) adminOperation(c *gin.Context, operation string) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "实例 ID 不合法"))
		return
	}
	var input AdminTransitionInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	input.IP = c.ClientIP()
	input.UserAgent = c.Request.UserAgent()
	item, err := h.service.AdminTransition(c.Request.Context(), actor, id, operation, c.GetString("idempotency_key"), httpx.RequestID(c), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, item)
}
func (h *Handler) AdminLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "实例 ID 不合法"))
		return
	}
	events, err := h.service.AdminLogs(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, events)
}
func (h *Handler) AdminWorkers(c *gin.Context) {
	items, err := h.service.AdminWorkers(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) AdminSetWorkerDrain(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input AdminWorkerDrainInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	input.IP = c.ClientIP()
	input.UserAgent = c.Request.UserAgent()
	item, err := h.service.AdminSetWorkerDrain(c.Request.Context(), actor, c.Param("workerId"), httpx.RequestID(c), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func scopeFromQuery(c *gin.Context) (Scope, error) {
	scope := Scope{}
	if raw := c.Query("competitionId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return Scope{}, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "比赛 ID 不合法")
		}
		scope.CompetitionID = &id
	}
	if raw := c.Query("teamId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return Scope{}, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "战队 ID 不合法")
		}
		scope.TeamID = &id
	}
	return scope, nil
}
