package cache

import (
	"context"
	"fmt"
	"time"

	"asamu.local/platform/api/internal/config"
	"github.com/redis/go-redis/v9"
)

type Redis struct{ Client *redis.Client }

func Open(cfg config.Redis) (*Redis, error) {
	client := redis.NewClient(&redis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB, DialTimeout: 3 * time.Second})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Redis{Client: client}, nil
}
func (r *Redis) Close() error                    { return r.Client.Close() }
func (r *Redis) Ready(ctx context.Context) error { return r.Client.Ping(ctx).Err() }

func (r *Redis) AcquireLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.Client.SetNX(ctx, "lock:"+key, value, ttl).Result()
}
func (r *Redis) ReleaseLock(ctx context.Context, key, value string) error {
	script := redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
	return script.Run(ctx, r.Client, []string{"lock:" + key}, value).Err()
}
