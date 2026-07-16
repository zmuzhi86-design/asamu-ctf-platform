package user

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) PublicProfile(c *gin.Context) {
	profile, err := h.service.Profile(c.Request.Context(), c.Param("id"), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, profile)
}
func (h *Handler) Me(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	profile, err := h.service.Profile(c.Request.Context(), userID.String(), true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, profile)
}
func (h *Handler) UpdateMe(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input Update
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	profile, err := h.service.Update(c.Request.Context(), userID, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, profile)
}
