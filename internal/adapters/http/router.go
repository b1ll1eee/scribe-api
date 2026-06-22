package http

import (
	"net/http"

	"scribe/backend/internal/adapters/repository/redis"
	"scribe/backend/internal/ports"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Router struct {
	handler    *HttpHandler
	jwtService ports.TokenService
	redisCache *redis.RedisCache
	logger     *zap.Logger
	corsOrigins []string
}

func NewRouter(handler *HttpHandler, jwtService ports.TokenService, cache *redis.RedisCache, logger *zap.Logger, corsOrigins []string) *Router {
	return &Router{
		handler:     handler,
		jwtService:  jwtService,
		redisCache:  cache,
		logger:      logger,
		corsOrigins: corsOrigins,
	}
}

func (r *Router) SetupEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	engine.Use(CorrelationIDMiddleware())
	engine.Use(StructuredLoggerMiddleware(r.logger))
	engine.Use(CORSMiddleware(r.corsOrigins))
	engine.Use(SecurityHeadersMiddleware())
	engine.Use(RateLimitMiddleware(r.redisCache, r.logger))
	engine.Use(gin.Recovery())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	engine.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "READY"})
	})

	apiV1 := engine.Group("/api/v1")
	{
		auth := apiV1.Group("/auth")
		{
			auth.POST("/login", r.handler.Login)
			auth.POST("/refresh", r.handler.Refresh)
			auth.POST("/logout", r.handler.Logout)
		}

		users := apiV1.Group("/users")
		users.Use(JWTMiddleware(r.jwtService))
		{
			users.GET("/me", r.handler.Me)
		}

		scribes := apiV1.Group("/scribes")
		scribes.Use(JWTMiddleware(r.jwtService))
		{
			scribes.POST("", r.handler.CreateScribe)
			scribes.GET("", r.handler.ListScribes)
			scribes.GET("/:id", r.handler.GetScribe)
			scribes.PUT("/:id", r.handler.UpdateScribe)
			scribes.DELETE("/:id", r.handler.DeleteScribe)
			scribes.POST("/:id/archive", r.handler.ArchiveScribe)
			scribes.POST("/:id/restore", r.handler.RestoreScribe)
			scribes.POST("/:id/pin", r.handler.PinScribe)
		}

		analytics := apiV1.Group("/analytics")
		analytics.Use(JWTMiddleware(r.jwtService))
		{
			analytics.GET("/dashboard", r.handler.GetDashboard)
		}
	}

	return engine
}
