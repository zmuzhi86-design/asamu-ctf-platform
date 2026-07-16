package queue

import (
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDecodeMarksMalformedMessagesAndPreservesTheirID(t *testing.T) {
	for _, message := range []redis.XMessage{
		{ID: "1-0", Values: map[string]any{}},
		{ID: "2-0", Values: map[string]any{"job": "{"}},
	} {
		_, messageID, err := decode(message)
		if messageID != message.ID {
			t.Fatalf("decode returned message ID %q, want %q", messageID, message.ID)
		}
		if !errors.Is(err, ErrMalformedJob) {
			t.Fatalf("decode error %v is not ErrMalformedJob", err)
		}
	}
}
