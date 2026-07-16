package submission

import (
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Submit(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var input SubmitInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	result, err := h.service.Submit(c.Request.Context(), userID, c.Param("id"), input, c.ClientIP(), c.Request.UserAgent(), httpx.RequestID(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) History(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, size := httpx.PageParams(c)
	items, err := h.service.History(c.Request.Context(), userID, c.Param("id"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
