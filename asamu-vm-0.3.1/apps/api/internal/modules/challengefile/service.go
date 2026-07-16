package challengefile

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"asamu.local/platform/api/internal/models"
	"asamu.local/platform/api/internal/platform/httpx"
	"asamu.local/platform/api/internal/platform/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const MaxUploadSize int64 = 64 << 20

type Service struct {
	db      *gorm.DB
	storage storage.Storage
}

func New(db *gorm.DB, store storage.Storage) *Service { return &Service{db: db, storage: store} }

type Record struct {
	ID                     uuid.UUID `json:"id"`
	Name, MIMEType, SHA256 string
	Size                   int64
	Public                 bool
	CreatedAt              time.Time
}
type Upload struct {
	Name, ClaimedType string
	Data              []byte
	Public            bool
}

func (s *Service) Upload(ctx context.Context, challengeKey string, input Upload) (Record, error) {
	var challenge models.Challenge
	if err := s.db.WithContext(ctx).Where("id::text=? OR slug=?", challengeKey, challengeKey).First(&challenge).Error; err != nil {
		return Record{}, httpx.NewError(http.StatusNotFound, "CHALLENGE_NOT_FOUND", "题目不存在")
	}
	name, err := safeName(input.Name)
	if err != nil {
		return Record{}, err
	}
	if len(input.Data) == 0 || int64(len(input.Data)) > MaxUploadSize {
		return Record{}, httpx.NewError(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "附件必须小于 64 MB")
	}
	detected := http.DetectContentType(input.Data[:min(512, len(input.Data))])
	if blockedType(detected, filepath.Ext(name)) {
		return Record{}, httpx.NewError(http.StatusUnsupportedMediaType, "UNSAFE_FILE_TYPE", "不允许上传可在浏览器直接执行的附件类型")
	}
	digest := sha256.Sum256(input.Data)
	hash := hex.EncodeToString(digest[:])
	var existing models.ChallengeFile
	if err := s.db.WithContext(ctx).Where("challenge_id=? AND name=? AND sha256=? AND archived_at IS NULL", challenge.ID, name, hash).Take(&existing).Error; err == nil {
		return record(existing), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Record{}, err
	}
	id := uuid.New()
	extension := strings.ToLower(filepath.Ext(name))
	objectKey := "challenge-files/" + challenge.ID.String() + "/" + id.String() + extension
	if err := s.storage.Put(ctx, objectKey, bytes.NewReader(input.Data), int64(len(input.Data)), detected); err != nil {
		return Record{}, err
	}
	model := models.ChallengeFile{ID: id, ChallengeID: challenge.ID, Name: name, ObjectKey: objectKey, MIMEType: detected, SHA256: hash, Size: int64(len(input.Data)), Public: input.Public, CreatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return tx.Model(&challenge).Updates(map[string]any{"has_attachment": true, "updated_at": time.Now().UTC()}).Error
	}); err != nil {
		_ = s.storage.Delete(ctx, objectKey)
		return Record{}, err
	}
	return record(model), nil
}

func (s *Service) Open(ctx context.Context, challengeKey string, fileID uuid.UUID, admin bool) (io.ReadCloser, Record, error) {
	var file models.ChallengeFile
	q := s.db.WithContext(ctx).Model(&models.ChallengeFile{}).Joins("JOIN challenges requested_challenge ON requested_challenge.id=challenge_files.challenge_id").Where("challenge_files.id=? AND (requested_challenge.id::text=? OR requested_challenge.slug=?)", fileID, challengeKey, challengeKey)
	if !admin {
		q = q.Where(`EXISTS (SELECT 1 FROM challenges c JOIN challenge_file_revisions r ON r.challenge_revision_id=c.current_published_revision_id WHERE c.id=challenge_files.challenge_id AND c.status='published' AND r.source_file_id=challenge_files.id)`)
	}
	if err := q.Take(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, Record{}, httpx.NewError(http.StatusNotFound, "FILE_NOT_FOUND", "附件不存在")
		}
		return nil, Record{}, err
	}
	reader, err := s.storage.Open(ctx, file.ObjectKey)
	if err != nil {
		return nil, Record{}, err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, file.Size+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, Record{}, readErr
	}
	if closeErr != nil {
		return nil, Record{}, closeErr
	}
	digest := sha256.Sum256(data)
	if int64(len(data)) != file.Size || !strings.EqualFold(hex.EncodeToString(digest[:]), file.SHA256) {
		return nil, Record{}, httpx.NewError(http.StatusInternalServerError, "FILE_INTEGRITY_FAILED", "附件完整性校验失败")
	}
	return io.NopCloser(bytes.NewReader(data)), record(file), nil
}

func (s *Service) Delete(ctx context.Context, fileID uuid.UUID) error {
	var file models.ChallengeFile
	if err := s.db.WithContext(ctx).First(&file, "id=?", fileID).Error; err != nil {
		return httpx.NewError(http.StatusNotFound, "FILE_NOT_FOUND", "附件不存在")
	}
	var revisionCount int64
	if err := s.db.WithContext(ctx).Table("challenge_file_revisions").Where("source_file_id=?", fileID).Count(&revisionCount).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if revisionCount > 0 {
			if err := tx.Model(&file).Update("archived_at", time.Now().UTC()).Error; err != nil {
				return err
			}
		} else if err := tx.Delete(&file).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.ChallengeFile{}).Where("challenge_id=? AND archived_at IS NULL", file.ChallengeID).Count(&count).Error; err != nil {
			return err
		}
		return tx.Table("challenges").Where("id=?", file.ChallengeID).Update("has_attachment", count > 0).Error
	}); err != nil {
		return err
	}
	if revisionCount == 0 {
		_ = s.storage.Delete(ctx, file.ObjectKey)
	}
	return nil
}

func safeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, `/\`) || value != filepath.Base(value) || len(value) > 180 {
		return "", httpx.NewError(http.StatusBadRequest, "INVALID_FILE_NAME", "附件名称不合法")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", httpx.NewError(http.StatusBadRequest, "INVALID_FILE_NAME", "附件名称不合法")
		}
	}
	return value, nil
}
func blockedType(contentType, extension string) bool {
	switch strings.ToLower(strings.Split(contentType, ";")[0]) {
	case "text/html", "image/svg+xml", "application/xhtml+xml", "application/javascript", "text/javascript", "application/x-shockwave-flash":
		return true
	}
	switch strings.ToLower(extension) {
	case ".html", ".htm", ".svg", ".js", ".mjs":
		return true
	}
	return false
}
func record(value models.ChallengeFile) Record {
	return Record{ID: value.ID, Name: value.Name, MIMEType: value.MIMEType, SHA256: value.SHA256, Size: value.Size, Public: value.Public, CreatedAt: value.CreatedAt}
}
func Disposition(name string) string {
	value := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if value == "" {
		return "attachment"
	}
	return value
}
