package auth

import (
	"net/http"
	"time"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service      *Service
	secureCookie bool
}

func NewHandler(service *Service, secureCookie bool) *Handler {
	return &Handler{service: service, secureCookie: secureCookie}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type loginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type passwordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}
type tokenRequest struct {
	Token string `json:"token" binding:"required"`
}
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}
type resetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}
type emailChangeRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewEmail        string `json:"newEmail" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	session, err := h.service.Register(c.Request.Context(), req.Email, req.Username, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, session)
	httpx.Created(c, session)
}
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	session, err := h.service.Login(c.Request.Context(), req.Login, req.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, session)
	httpx.OK(c, session)
}
func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie("cm_refresh")
	session, err := h.service.Refresh(c.Request.Context(), raw, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		h.clearCookie(c)
		httpx.Fail(c, err)
		return
	}
	h.setCookie(c, session)
	httpx.OK(c, session)
}
func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie("cm_refresh")
	if err := h.service.Logout(c.Request.Context(), raw, false); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearCookie(c)
	httpx.NoContent(c)
}
func (h *Handler) LogoutAll(c *gin.Context) {
	raw, _ := c.Cookie("cm_refresh")
	if err := h.service.Logout(c.Request.Context(), raw, true); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearCookie(c)
	httpx.NoContent(c)
}
func (h *Handler) Me(c *gin.Context) {
	id, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	user, err := h.service.Me(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, user)
}
func (h *Handler) ChangePassword(c *gin.Context) {
	id, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req passwordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), id, req.CurrentPassword, req.NewPassword); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearCookie(c)
	httpx.NoContent(c)
}
func (h *Handler) ResendVerification(c *gin.Context) {
	id, err := httpx.UserID(c)
	if err == nil {
		err = h.service.ResendVerification(c.Request.Context(), id)
	}
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, map[string]string{"message": "如需验证，邮件已进入发送队列"})
}
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req tokenRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, map[string]bool{"verified": true})
}
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, map[string]string{"message": "如果账户存在，重置邮件已进入发送队列"})
}
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearCookie(c)
	httpx.NoContent(c)
}
func (h *Handler) RequestEmailChange(c *gin.Context) {
	id, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	var req emailChangeRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.RequestEmailChange(c.Request.Context(), id, req.CurrentPassword, req.NewEmail); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, map[string]string{"message": "确认邮件已进入发送队列"})
}
func (h *Handler) ConfirmEmailChange(c *gin.Context) {
	var req tokenRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.service.ConfirmEmailChange(c.Request.Context(), req.Token); err != nil {
		httpx.Fail(c, err)
		return
	}
	h.clearCookie(c)
	httpx.OK(c, map[string]bool{"changed": true})
}
func (h *Handler) setCookie(c *gin.Context, session Session) {
	maxAge := int(time.Until(session.RefreshTokenExpiresAt).Seconds())
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("cm_refresh", session.RefreshToken, maxAge, "/api/v1/auth", "", h.secureCookie, true)
}
func (h *Handler) clearCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("cm_refresh", "", -1, "/api/v1/auth", "", h.secureCookie, true)
}
