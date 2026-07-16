package httpx

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic_recovered", zap.Any("error", recovered), zap.ByteString("stack", debug.Stack()), zap.String("request_id", RequestID(c)))
				Fail(c, NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "服务器处理请求时发生错误"))
			}
		}()
		c.Next()
	}
}
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func CSRFOrigins(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[strings.TrimRight(origin, "/")] = true
	}
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
		if origin == "" {
			referer := strings.TrimSpace(c.GetHeader("Referer"))
			for candidate := range allowed {
				if strings.HasPrefix(referer, candidate+"/") {
					origin = candidate
					break
				}
			}
		}
		if origin == "" && c.GetHeader("Cookie") == "" {
			c.Next()
			return
		}
		if !allowed[origin] {
			Fail(c, NewError(http.StatusForbidden, "CSRF_ORIGIN_DENIED", "请求来源校验失败"))
			return
		}
		c.Next()
	}
}
func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID, X-Confirmation-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

type RateLimiter struct {
	client *redis.Client
	rps    float64
	burst  int
	script *redis.Script
}

func NewRateLimiter(client *redis.Client, rps float64, burst int) *RateLimiter {
	return &RateLimiter{client: client, rps: rps, burst: burst, script: redis.NewScript(`
local now=tonumber(ARGV[1])
local rate=tonumber(ARGV[2])
local burst=tonumber(ARGV[3])
local ttl=tonumber(ARGV[4])
local values=redis.call('HMGET',KEYS[1],'tokens','updated_at')
local tokens=tonumber(values[1]) or burst
local updated=tonumber(values[2]) or now
tokens=math.min(burst,tokens+math.max(0,now-updated)*rate/1000)
local allowed=0
if tokens>=1 then tokens=tokens-1 allowed=1 end
redis.call('HSET',KEYS[1],'tokens',tokens,'updated_at',now)
redis.call('PEXPIRE',KEYS[1],ttl)
return allowed`)}
}
func (l *RateLimiter) Middleware(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			identity = fmt.Sprint(userID)
		}
		ttl := time.Duration(float64(2*l.burst) / l.rps * float64(time.Second))
		if ttl < time.Second {
			ttl = time.Second
		}
		allowed, err := l.script.Run(c.Request.Context(), l.client, []string{"rate:" + scope + ":" + identity}, time.Now().UnixMilli(), l.rps, l.burst, ttl.Milliseconds()).Int()
		if err == nil && allowed == 0 {
			c.Header("Retry-After", "1")
			Fail(c, NewError(http.StatusTooManyRequests, "RATE_LIMITED", "请求过于频繁，请稍后重试"))
			return
		}
		c.Next()
	}
}
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if len(key) < 8 || len(key) > 128 {
			Fail(c, NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "该操作需要有效的 Idempotency-Key"))
			return
		}
		c.Set("idempotency_key", key)
		c.Next()
	}
}
func UserID(c *gin.Context) (uuid.UUID, error) {
	value, ok := c.Get("user_id")
	if !ok {
		return uuid.Nil, NewError(http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录")
	}
	id, ok := value.(uuid.UUID)
	if !ok || id == uuid.Nil {
		return uuid.Nil, NewError(http.StatusUnauthorized, "INVALID_SESSION", "登录会话无效")
	}
	return id, nil
}
