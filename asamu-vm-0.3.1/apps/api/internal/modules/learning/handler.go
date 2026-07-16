package learning

import (
	"net/http"
	"regexp"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func optionalUserID(c *gin.Context) *uuid.UUID {
	id, err := httpx.UserID(c)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context(), optionalUserID(c), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}

func (h *Handler) Detail(c *gin.Context) {
	item, err := h.service.Detail(c.Request.Context(), c.Param("id"), optionalUserID(c), false)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) AdminList(c *gin.Context) {
	items, err := h.service.List(c.Request.Context(), nil, true)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}

func (h *Handler) Save(c *gin.Context) {
	var input Mutation
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	if !slugPattern.MatchString(input.Slug) {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_SLUG", "路线 Slug 只能包含小写字母、数字和连字符"))
		return
	}
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Save(c.Request.Context(), c.Param("id"), input, actor)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	if c.Param("id") == "" {
		httpx.Created(c, item)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) Publish(c *gin.Context) {
	item, err := h.service.Publish(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) Archive(c *gin.Context) {
	if err := h.service.Archive(c.Request.Context(), c.Param("id")); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
