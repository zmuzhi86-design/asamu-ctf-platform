package challengefile

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(c, httpx.NewError(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "附件必须小于 64 MB"))
			return
		}
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "FILE_REQUIRED", "请选择附件"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxUploadSize+1))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	public, _ := strconv.ParseBool(c.PostForm("public"))
	item, err := h.service.Upload(c.Request.Context(), c.Param("id"), Upload{Name: header.Filename, ClaimedType: header.Header.Get("Content-Type"), Data: data, Public: public})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Download(c *gin.Context)      { h.download(c, false) }
func (h *Handler) AdminDownload(c *gin.Context) { h.download(c, true) }
func (h *Handler) download(c *gin.Context, admin bool) {
	id, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "附件 ID 不合法"))
		return
	}
	reader, item, err := h.service.Open(c.Request.Context(), c.Param("id"), id, admin)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", item.MIMEType)
	c.Header("Content-Disposition", Disposition(item.Name))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private,no-store")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "附件 ID 不合法"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
