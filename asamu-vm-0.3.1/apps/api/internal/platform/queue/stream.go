package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrMalformedJob = errors.New("malformed stream job")

type Job struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Attempts  int             `json:"attempts"`
	CreatedAt time.Time       `json:"createdAt"`
}
type Stream struct {
	client                  *redis.Client
	stream, group, consumer string
}

func NewStream(client *redis.Client, stream, group, consumer string) *Stream {
	return &Stream{client: client, stream: stream, group: group, consumer: consumer}
}
func (s *Stream) Ensure(ctx context.Context) error {
	err := s.client.XGroupCreateMkStream(ctx, s.stream, s.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}
func (s *Stream) Publish(ctx context.Context, job Job) (string, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return "", err
	}
	return s.client.XAdd(ctx, &redis.XAddArgs{Stream: s.stream, Values: map[string]any{"job": string(payload)}}).Result()
}
func (s *Stream) Receive(ctx context.Context, block time.Duration) (Job, string, error) {
	result, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: s.group, Consumer: s.consumer, Streams: []string{s.stream, ">"}, Count: 1, Block: block}).Result()
	if errors.Is(err, redis.Nil) {
		return Job{}, "", nil
	}
	if err != nil {
		return Job{}, "", err
	}
	message := result[0].Messages[0]
	return decode(message)
}

func (s *Stream) ClaimStale(ctx context.Context, minIdle time.Duration) (Job, string, error) {
	messages, _, err := s.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: s.stream, Group: s.group, Consumer: s.consumer, MinIdle: minIdle, Start: "0-0", Count: 1}).Result()
	if errors.Is(err, redis.Nil) || (err == nil && len(messages) == 0) {
		return Job{}, "", nil
	}
	if err != nil {
		return Job{}, "", err
	}
	return decode(messages[0])
}

func decode(message redis.XMessage) (Job, string, error) {
	raw, ok := message.Values["job"].(string)
	if !ok {
		return Job{}, message.ID, fmt.Errorf("%w: missing job payload", ErrMalformedJob)
	}
	var job Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return Job{}, message.ID, fmt.Errorf("%w: %v", ErrMalformedJob, err)
	}
	return job, message.ID, nil
}
func (s *Stream) Ack(ctx context.Context, messageID string) error {
	if messageID == "" {
		return nil
	}
	return s.client.XAck(ctx, s.stream, s.group, messageID).Err()
}
func (s *Stream) Retry(ctx context.Context, job Job, messageID string, delay time.Duration) error {
	job.Attempts++
	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if _, err := s.Publish(ctx, job); err != nil {
		return err
	}
	return s.Ack(ctx, messageID)
}

// Requeue hands a job back to the consumer group without consuming a retry
// attempt. It is used when a healthy worker is intentionally drained or full.
func (s *Stream) Requeue(ctx context.Context, job Job, messageID string, delay time.Duration) error {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if _, err := s.Publish(ctx, job); err != nil {
		return err
	}
	return s.Ack(ctx, messageID)
}

func (s *Stream) DeadLetter(ctx context.Context, job Job, messageID, code string) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if err := s.client.XAdd(ctx, &redis.XAddArgs{Stream: s.stream + ".dead", Values: map[string]any{"job": string(payload), "sourceMessageId": messageID, "errorCode": code, "failedAt": time.Now().UTC().Format(time.RFC3339Nano)}}).Err(); err != nil {
		return err
	}
	return s.Ack(ctx, messageID)
}
