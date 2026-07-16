package scoreboard

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Global(c *gin.Context) {
	page, size := httpx.PageParams(c)
	rows, err := h.service.Global(c.Request.Context(), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}
func (h *Handler) Competition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "比赛 ID 不合法"))
		return
	}
	page, size := httpx.PageParams(c)
	rows, err := h.service.Competition(c.Request.Context(), id, page, size, true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}

func (h *Handler) VoidEvent(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "积分事件 ID 不合法"))
		return
	}
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	result, err := h.service.VoidEvent(c.Request.Context(), eventID, actor, input.Reason)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) Rebuild(c *gin.Context) {
	result, err := h.service.RebuildDerived(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) Adjust(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input AdjustmentInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	result, err := h.service.Adjust(c.Request.Context(), actor, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, result)
}

func (h *Handler) Events(c *gin.Context) {
	var userID, competitionID *uuid.UUID
	for key, target := range map[string]**uuid.UUID{"userId": &userID, "competitionId": &competitionID} {
		if raw := c.Query(key); raw != "" {
			value, err := uuid.Parse(raw)
			if err != nil {
				httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", key+" 不合法"))
				return
			}
			*target = &value
		}
	}
	page, size := httpx.PageParams(c)
	rows, err := h.service.Events(c.Request.Context(), userID, competitionID, c.Query("type"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, rows)
}
