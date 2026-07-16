package audit

import (
	"encoding/json"

	"asamu.local/platform/api/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

type Entry struct {
	ActorID                                     *uuid.UUID
	ActorType, Action, ResourceType, ResourceID string
	Before, After                               any
}

func (s *Service) Record(c *gin.Context, entry Entry) error {
	before, _ := json.Marshal(entry.Before)
	after, _ := json.Marshal(entry.After)
	return s.db.WithContext(c.Request.Context()).Create(&models.AuditLog{ID: uuid.New(), ActorID: entry.ActorID, ActorType: entry.ActorType, Action: entry.Action, ResourceType: entry.ResourceType, ResourceID: entry.ResourceID, BeforeJSON: before, AfterJSON: after, IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), RequestID: c.GetString("request_id")}).Error
}
