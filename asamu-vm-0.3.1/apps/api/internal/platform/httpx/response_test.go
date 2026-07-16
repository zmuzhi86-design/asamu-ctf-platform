package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPageMarshalUsesEmptyArrayForNilItems(t *testing.T) {
	payload, err := json.Marshal(Page[string]{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Items == nil || len(decoded.Items) != 0 {
		t.Fatalf("expected a non-nil empty items array, got %#v", decoded.Items)
	}
}

func TestBindJSONRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	body := `{"value":"` + strings.Repeat("x", int(MaxJSONBodySize)) + `"}`
	context.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	var target map[string]string
	err := BindJSON(context, &target)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected request-too-large error, got %#v", err)
	}
}
