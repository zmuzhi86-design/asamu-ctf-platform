package registrycredential

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service     *Service
	workerToken string
}

func NewHandler(service *Service, workerToken string) *Handler {
	return &Handler{service: service, workerToken: workerToken}
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}

func (h *Handler) Create(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var body struct{ Name, RegistryHost, Username, Token string }
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), actor, CreateInput{Name: body.Name, RegistryHost: body.RegistryHost, Username: body.Username, Token: body.Token, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), RequestID: httpx.RequestID(c)})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}

func (h *Handler) Update(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "凭据 ID 不合法"))
		return
	}
	var body struct {
		Name, Username, Token, Reason string
		Enabled                       *bool
		ExpectedVersion               int64
	}
	if err := httpx.BindJSON(c, &body); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), actor, id, UpdateInput{Name: body.Name, Username: body.Username, Token: body.Token, Reason: body.Reason, Enabled: body.Enabled, ExpectedVersion: body.ExpectedVersion, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), RequestID: httpx.RequestID(c)})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) Lease(c *gin.Context) {
	if !secureEqual(c.GetHeader("X-Worker-Token"), h.workerToken) {
		httpx.Fail(c, httpx.NewError(http.StatusUnauthorized, "WORKER_UNAUTHORIZED", "Worker 认证失败"))
		return
	}
	credentialID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "凭据 ID 不合法"))
		return
	}
	var body struct {
		InstanceID uuid.UUID `json:"instanceId"`
	}
	if err := httpx.BindJSON(c, &body); err != nil || body.InstanceID == uuid.Nil {
		if err == nil {
			err = httpx.NewError(http.StatusBadRequest, "INVALID_ID", "实例 ID 不合法")
		}
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Lease(c.Request.Context(), credentialID, body.InstanceID, c.GetHeader("X-Worker-ID"), httpx.RequestID(c), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	httpx.OK(c, item)
}

func secureEqual(actual, expected string) bool {
	actualHash, expectedHash := sha256.Sum256([]byte(actual)), sha256.Sum256([]byte(expected))
	return expected != "" && subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}
