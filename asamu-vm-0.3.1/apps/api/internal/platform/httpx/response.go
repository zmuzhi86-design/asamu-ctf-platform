package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Status        int
	Code, Message string
	Details       any
}

const MaxJSONBodySize int64 = 2 << 20

func (e *Error) Error() string { return e.Message }
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

type Envelope struct {
	Success   bool       `json:"success"`
	Data      any        `json:"data,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	RequestID string     `json:"requestId"`
}
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}
type Page[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

func (p Page[T]) MarshalJSON() ([]byte, error) {
	items := p.Items
	if items == nil {
		items = make([]T, 0)
	}
	type pageJSON struct {
		Items      []T   `json:"items"`
		Page       int   `json:"page"`
		PageSize   int   `json:"pageSize"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
	}
	return json.Marshal(pageJSON{Items: items, Page: p.Page, PageSize: p.PageSize, Total: p.Total, TotalPages: p.TotalPages})
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data, RequestID: RequestID(c)})
}
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data, RequestID: RequestID(c)})
}
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, Envelope{Success: true, Data: data, RequestID: RequestID(c)})
}
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }
func Fail(c *gin.Context, err error) {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		apiErr = NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器处理请求时发生错误")
	}
	c.AbortWithStatusJSON(apiErr.Status, Envelope{Success: false, Error: &ErrorBody{Code: apiErr.Code, Message: apiErr.Message, Details: apiErr.Details}, RequestID: RequestID(c)})
}
func BindJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBodySize)
	if err := c.ShouldBindJSON(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return &Error{Status: http.StatusRequestEntityTooLarge, Code: "REQUEST_TOO_LARGE", Message: "请求内容过大"}
		}
		return &Error{Status: http.StatusBadRequest, Code: "INVALID_REQUEST", Message: "请求参数不合法", Details: err.Error()}
	}
	return nil
}
func RequestID(c *gin.Context) string {
	value, _ := c.Get("request_id")
	if id, ok := value.(string); ok {
		return id
	}
	return ""
}
func PageParams(c *gin.Context) (int, int) {
	page, size := 1, 20
	if _, err := fmtSscan(c.Query("page"), &page); err != nil || page < 1 {
		page = 1
	}
	if _, err := fmtSscan(c.Query("pageSize"), &size); err != nil || size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func fmtSscan(value string, target *int) (int, error) {
	if value == "" {
		return 0, errors.New("empty")
	}
	var parsed int
	_, err := fmt.Sscan(value, &parsed)
	if err == nil {
		*target = parsed
	}
	return 1, err
}
