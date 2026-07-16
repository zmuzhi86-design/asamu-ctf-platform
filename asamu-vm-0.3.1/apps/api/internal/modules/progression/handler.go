package progression

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Me(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	profile, err := h.service.Profile(c.Request.Context(), userID, c.Query("scheme"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, profile)
}
func (h *Handler) CreateScheme(c *gin.Context) {
	var input struct {
		Key, Name string
		Tiers     []Tier
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.CreateScheme(c.Request.Context(), input.Key, input.Name, input.Tiers); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
