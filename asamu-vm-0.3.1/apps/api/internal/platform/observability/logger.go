package observability

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func NewLogger(environment string) (*zap.Logger, error) {
	if environment == "development" || environment == "test" {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}

func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		started := time.Now()
		c.Next()
		logger.Info("http_request", zap.String("request_id", requestID), zap.String("method", c.Request.Method), zap.String("path", c.Request.URL.Path), zap.Int("status", c.Writer.Status()), zap.Duration("duration", time.Since(started)), zap.String("client_ip", c.ClientIP()), zap.String("user_agent", c.Request.UserAgent()))
	}
}

type contextKey string

const loggerKey contextKey = "logger"

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
func FromContext(ctx context.Context) *zap.Logger {
	if value, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return value
	}
	return zap.NewNop()
}
