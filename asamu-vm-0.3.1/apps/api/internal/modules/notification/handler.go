package notification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), userID, page, size, c.Query("unread") == "true")
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Read(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "通知 ID 不合法"))
		return
	}
	if err := h.service.Read(c.Request.Context(), userID, id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) ReadAll(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ReadAll(c.Request.Context(), userID); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Events(c *gin.Context) {
	userID, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		httpx.Fail(c, httpx.NewError(http.StatusInternalServerError, "SSE_UNSUPPORTED", "服务端不支持事件流"))
		return
	}
	send := func(event models.Notification) {
		payload, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(c.Writer, "id: %s\nevent: notification\ndata: %s\n\n", event.ID, payload)
		flusher.Flush()
	}
	pending, err := h.service.EventsAfter(c.Request.Context(), userID, c.GetHeader("Last-Event-ID"))
	if err == nil {
		for _, event := range pending {
			send(event)
		}
	}
	subscription := h.service.Subscribe(c.Request.Context(), userID)
	defer subscription.Close()
	channel := subscription.Channel()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		case message, open := <-channel:
			if !open {
				return
			}
			var event models.Notification
			if json.Unmarshal([]byte(message.Payload), &event) == nil {
				send(event)
			}
		}
	}
}
