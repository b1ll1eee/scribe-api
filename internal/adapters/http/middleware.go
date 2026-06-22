package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"scribe/backend/internal/adapters/repository/redis"
	"scribe/backend/internal/ports"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func CorrelationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		corID := c.GetHeader("X-Correlation-ID")
		if corID == "" {
			corID = uuid.NewString()
		}
		c.Set("correlation_id", corID)
		c.Header("X-Correlation-ID", corID)
		c.Next()
	}
}

func StructuredLoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		corID, _ := c.Get("correlation_id")

		logger.Info("http request",
			zap.String("correlation_id", corID.(string)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", duration),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Correlation-ID")
		c.Header("Access-Control-Allow-Methods", "POST, HEAD, PATCH, OPTIONS, GET, PUT, DELETE")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

func JWTMiddleware(jwtService ports.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			jsonErr(c, http.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			jsonErr(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization header must be Bearer <token>")
			c.Abort()
			return
		}

		userProfile, err := jwtService.ValidateAccessToken(parts[1])
		if err != nil {
			jsonErr(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired access token")
			c.Abort()
			return
		}

		c.Set("user", userProfile)
		c.Next()
	}
}

func RateLimitMiddleware(cache *redis.RedisCache, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache == nil {
			c.Next()
			return
		}

		ip := c.ClientIP()
		key := "ratelimit:" + ip
		limit := int64(120)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		count, err := cache.Increment(ctx, key, 1*time.Minute)
		if err != nil {
			logger.Warn("rate limiter redis error", zap.Error(err))
			c.Next()
			return
		}

		if count > limit {
			jsonErr(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests, try again in a minute")
			c.Abort()
			return
		}
		c.Next()
	}
}
