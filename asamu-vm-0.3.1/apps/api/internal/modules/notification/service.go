package notification

import (
	"context"
	"encoding/json"
	"time"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	db    *gorm.DB
	redis *redis.Client
}

func New(db *gorm.DB, redisClient *redis.Client) *Service {
	return &Service{db: db, redis: redisClient}
}
func (s *Service) Create(ctx context.Context, userID uuid.UUID, eventType, title, body, link string, payload any) (models.Notification, error) {
	raw, _ := json.Marshal(payload)
	notification := models.Notification{ID: uuid.New(), UserID: userID, Type: eventType, Title: title, Body: body, Link: link, Payload: raw, CreatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Create(&notification).Error; err != nil {
		return models.Notification{}, err
	}
	encoded, _ := json.Marshal(notification)
	_ = s.redis.Publish(ctx, "notifications:"+userID.String(), encoded).Err()
	return notification, nil
}
func (s *Service) List(ctx context.Context, userID uuid.UUID, page, size int, unreadOnly bool) (httpx.Page[models.Notification], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := s.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id=?", userID)
	if unreadOnly {
		query = query.Where("read_at IS NULL")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return httpx.Page[models.Notification]{}, err
	}
	var items []models.Notification
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return httpx.Page[models.Notification]{}, err
	}
	return httpx.Page[models.Notification]{Items: items, Page: page, PageSize: size, Total: total, TotalPages: int((total + int64(size) - 1) / int64(size))}, nil
}
func (s *Service) Read(ctx context.Context, userID, notificationID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&models.Notification{}).Where("id=? AND user_id=?", notificationID, userID).Update("read_at", time.Now().UTC()).Error
}
func (s *Service) ReadAll(ctx context.Context, userID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id=? AND read_at IS NULL", userID).Update("read_at", time.Now().UTC()).Error
}
func (s *Service) EventsAfter(ctx context.Context, userID uuid.UUID, lastID string) ([]models.Notification, error) {
	query := s.db.WithContext(ctx).Where("user_id=?", userID)
	if id, err := uuid.Parse(lastID); err == nil {
		var previous models.Notification
		if s.db.WithContext(ctx).First(&previous, "id=? AND user_id=?", id, userID).Error == nil {
			query = query.Where("created_at>?", previous.CreatedAt)
		}
	}
	var items []models.Notification
	err := query.Order("created_at").Limit(100).Find(&items).Error
	return items, err
}
func (s *Service) Subscribe(ctx context.Context, userID uuid.UUID) *redis.PubSub {
	return s.redis.Subscribe(ctx, "notifications:"+userID.String())
}
