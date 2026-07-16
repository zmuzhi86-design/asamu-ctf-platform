package asset

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) PublicManifest(c *gin.Context) {
	manifest, err := h.service.PublicManifest(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Header("Cache-Control", "public,max-age=60,stale-while-revalidate=300")
	httpx.OK(c, manifest)
}
func (h *Handler) Content(c *gin.Context) {
	id, err := uuid.Parse(c.Param("versionId"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材版本 ID 不合法"))
		return
	}
	reader, mime, err := h.service.OpenVersion(c.Request.Context(), id, c.Query("variant"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	defer reader.Close()
	c.Header("Content-Type", mime)
	c.Header("Cache-Control", "public,max-age=31536000,immutable")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}
func (h *Handler) List(c *gin.Context) {
	page, size := httpx.PageParams(c)
	items, err := h.service.List(c.Request.Context(), c.Query("search"), c.Query("category"), c.Query("status"), page, size)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材 ID 不合法"))
		return
	}
	var input CreateInput
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func (h *Handler) Create(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize+(1<<20))
	input, upload, err := parseMultipart(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), actor, input, upload)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) AddVersion(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材 ID 不合法"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Fail(c, httpx.NewError(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "素材文件不能超过 25 MB"))
			return
		}
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "FILE_REQUIRED", "请选择素材文件"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.AddVersion(c.Request.Context(), actor, id, Upload{Name: header.Filename, ContentType: header.Header.Get("Content-Type"), Data: data}, c.PostForm("note"))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Publish(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材 ID 不合法"))
		return
	}
	if err := h.service.Publish(c.Request.Context(), id, actor); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Rollback(c *gin.Context) {
	actor, err := httpx.UserID(c)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材 ID 不合法"))
		return
	}
	if err := h.service.Rollback(c.Request.Context(), id, actor); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Archive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "素材 ID 不合法"))
		return
	}
	if err := h.service.Archive(c.Request.Context(), id); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}
func (h *Handler) Categories(c *gin.Context) {
	items, err := h.service.Categories(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) CreateCategory(c *gin.Context) {
	var input struct{ Key, Name, Description string }
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.CreateCategory(c.Request.Context(), input.Key, input.Name, input.Description)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Tags(c *gin.Context) {
	items, err := h.service.Tags(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) CreateTag(c *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.CreateTag(c.Request.Context(), input.Name)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) Slots(c *gin.Context) {
	items, err := h.service.Slots(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, items)
}
func (h *Handler) CreateSlot(c *gin.Context) {
	var input Slot
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.CreateSlot(c.Request.Context(), input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, item)
}
func (h *Handler) UpdateSlot(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.NewError(http.StatusBadRequest, "INVALID_ID", "槽位 ID 不合法"))
		return
	}
	var input Slot
	if err := httpx.BindJSON(c, &input); err != nil {
		httpx.Fail(c, err)
		return
	}
	item, err := h.service.UpdateSlot(c.Request.Context(), id, input)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, item)
}
func parseMultipart(c *gin.Context) (CreateInput, Upload, error) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return CreateInput{}, Upload{}, httpx.NewError(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "素材文件不能超过 25 MB")
		}
		return CreateInput{}, Upload{}, httpx.NewError(http.StatusBadRequest, "FILE_REQUIRED", "请选择素材文件")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return CreateInput{}, Upload{}, err
	}
	tags := split(c.PostForm("tags"))
	pages := split(c.PostForm("applicablePages"))
	input := CreateInput{AssetKey: c.PostForm("assetKey"), Name: c.PostForm("name"), Category: c.PostForm("category"), AltText: c.PostForm("altText"), Fit: c.PostForm("fit"), Position: c.PostForm("position"), FallbackAssetKey: c.PostForm("fallbackAssetKey"), Tags: tags, ApplicablePages: pages}
	return input, Upload{Name: header.Filename, ContentType: header.Header.Get("Content-Type"), Data: data}, nil
}
func split(value string) []string {
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
