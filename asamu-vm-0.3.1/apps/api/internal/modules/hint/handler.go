package hint

import (
	"net/http"
	"strconv"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type input struct{ CompetitionID, TeamID *uuid.UUID }

func (h *Handler) List(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	competitionID, teamID, err := params(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	items, err := h.service.List(c.Request.Context(), userID, c.Param("id"), competitionID, teamID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Unlock(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_HINT_INDEX", "Hint 序号不合法"))
		return
	}
	var value input
	if err := httpx.BindJSON(c, &value); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Unlock(c.Request.Context(), userID, c.Param("id"), index, value.CompetitionID, value.TeamID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func params(c *gin.Context) (*uuid.UUID, *uuid.UUID, error) {
	var competitionID, teamID *uuid.UUID
	for key, target := range map[string]**uuid.UUID{"competitionId": &competitionID, "teamId": &teamID} {
		if raw := c.Query(key); raw != "" {
			value, err := uuid.Parse(raw)
			if err != nil {
				return nil, nil, httpx.NewError(http.StatusBadRequest, "INVALID_ID", key+" 不合法")
			}
			*target = &value
		}
	}
	return competitionID, teamID, nil
}
