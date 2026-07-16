package mail

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"asamu.local/platform/api/internal/config"
	"asamu.local/platform/api/internal/platform/security"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Dispatcher struct {
	db     *gorm.DB
	cfg    config.Mail
	key    []byte
	logger *zap.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type delivery struct {
	ID                     uuid.UUID
	Recipient, TemplateKey string
	PayloadCiphertext      []byte
	Attempts               int
}

func NewDispatcher(db *gorm.DB, cfg config.Mail, confirmationSecret string, logger *zap.Logger) *Dispatcher {
	key := sha256.Sum256([]byte(confirmationSecret))
	return &Dispatcher{db: db, cfg: cfg, key: key[:], logger: logger}
}

func (d *Dispatcher) Start(parent context.Context) {
	if d.cfg.Driver == "disabled" {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	d.cancel = cancel
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(d.cfg.PollInterval)
		defer ticker.Stop()
		for {
			if err := d.dispatchOne(ctx); err != nil && ctx.Err() == nil {
				d.logger.Error("email outbox dispatch failed", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (d *Dispatcher) Close() {
	if d.cancel != nil {
		d.cancel()
		d.wg.Wait()
	}
}

func (d *Dispatcher) dispatchOne(ctx context.Context) error {
	var item delivery
	found := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		stale := now.Add(-5 * time.Minute)
		q := tx.Table("email_outbox").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("(status IN ('pending','failed') AND available_at<=?) OR (status='sending' AND locked_at<?)", now, stale).Order("created_at").Limit(1).Find(&item)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			return nil
		}
		found = true
		return tx.Table("email_outbox").Where("id=?", item.ID).Updates(map[string]any{"status": "sending", "locked_at": now, "attempts": gorm.Expr("attempts+1"), "updated_at": now}).Error
	})
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	item.Attempts++
	if err := d.send(item); err != nil {
		d.fail(item, err)
		return err
	}
	now := time.Now().UTC()
	return d.db.WithContext(ctx).Table("email_outbox").Where("id=?", item.ID).Updates(map[string]any{"status": "sent", "sent_at": now, "locked_at": nil, "last_error": nil, "updated_at": now}).Error
}

func (d *Dispatcher) send(item delivery) error {
	plaintext, err := security.Decrypt(item.PayloadCiphertext, d.key)
	if err != nil {
		return fmt.Errorf("decrypt outbox %s: %w", item.ID, err)
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return err
	}
	if payload.URL == "" {
		return fmt.Errorf("email payload URL is empty")
	}
	if d.cfg.Driver == "log" {
		d.logger.Info("email delivery simulated", zap.String("outbox_id", item.ID.String()), zap.String("recipient", item.Recipient), zap.String("template", item.TemplateKey))
		return nil
	}
	parsed, err := mail.ParseAddress(item.Recipient)
	if err != nil || !strings.EqualFold(parsed.Address, item.Recipient) {
		return fmt.Errorf("invalid recipient")
	}
	from, err := mail.ParseAddress(d.cfg.From)
	if err != nil || !strings.EqualFold(from.Address, d.cfg.From) {
		return fmt.Errorf("invalid sender")
	}
	subject, title := "asamu 邮件通知", "asamu"
	if item.TemplateKey == "verify_email" {
		subject, title = "验证你的 asamu 邮箱", "邮箱验证"
	}
	if item.TemplateKey == "reset_password" {
		subject, title = "重置你的 asamu 密码", "密码重置"
	}
	if item.TemplateKey == "change_email" {
		subject, title = "确认你的 asamu 新邮箱", "确认新邮箱"
	}
	if item.TemplateKey == "team_invitation" {
		subject, title = "你收到一封 asamu 战队邀请", "战队邀请"
	}
	body := "<!doctype html><html><body><h2>" + html.EscapeString(title) + "</h2><p><a href=\"" + html.EscapeString(payload.URL) + "\">继续操作</a></p><p>如果不是你发起的请求，请忽略此邮件。</p></body></html>"
	msg := []byte("From: " + from.Address + "\r\nTo: " + parsed.Address + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	var auth smtp.Auth
	if d.cfg.SMTPUsername != "" {
		auth = smtp.PlainAuth("", d.cfg.SMTPUsername, d.cfg.SMTPPassword, d.cfg.SMTPHost)
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", d.cfg.SMTPHost, d.cfg.SMTPPort), auth, from.Address, []string{parsed.Address}, msg)
}

func (d *Dispatcher) fail(item delivery, sendErr error) {
	now := time.Now().UTC()
	status := "failed"
	if item.Attempts >= 8 {
		status = "dead"
	}
	delay := time.Minute * time.Duration(1<<min(item.Attempts-1, 6))
	message := sendErr.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	_ = d.db.Table("email_outbox").Where("id=?", item.ID).Updates(map[string]any{"status": status, "available_at": now.Add(delay), "locked_at": nil, "last_error": message, "updated_at": now}).Error
}
