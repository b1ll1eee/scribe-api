package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"scribe/backend/internal/adapters/auth"
	httpAdapter "scribe/backend/internal/adapters/http"
	"scribe/backend/internal/adapters/repository/clickhouse"
	"scribe/backend/internal/adapters/repository/postgres"
	"scribe/backend/internal/adapters/repository/redis"
	"scribe/backend/internal/adapters/service"
	"scribe/backend/internal/config"
	"scribe/backend/internal/infrastructure/logger"
	"scribe/backend/internal/ports"
)

func main() {
	log := logger.New()
	defer func() { _ = log.Sync() }()

	cfg := config.Load()

	log.Info("starting scribe application api", zap.String("port", cfg.Server.Port))

	db, err := connectPostgres(cfg.Postgres, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	cache, err := redis.NewRedisCache(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Warn("redis unavailable, rate limiting disabled", zap.Error(err))
	} else {
		defer cache.Close()
		log.Info("redis connection established")
	}

	chRepo, err := clickhouse.NewClickHouseRepository(
		cfg.ClickHouse.Addr, cfg.ClickHouse.Database,
		cfg.ClickHouse.Username, cfg.ClickHouse.Password,
	)
	if err != nil {
		log.Fatal("failed to connect to clickhouse", zap.Error(err))
	}
	defer chRepo.Close()
	log.Info("clickhouse connection established")
	var analyticsRepo ports.AnalyticsRepository = chRepo

	pgRepo := postgres.NewPostgresRepository(db)
	scribeRepoAdapter := postgres.NewScribeRepositoryAdapter(pgRepo)

	firebaseVerifier, err := auth.NewFirebaseAuthService(cfg.Firebase.CredentialsJSON, cfg.Firebase.MockMode, log)
	if err != nil {
		log.Fatal("failed to initialize firebase auth", zap.Error(err))
	}

	jwtService := auth.NewJWTTokenService(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)

	authSvc := service.NewAuthService(pgRepo, pgRepo, firebaseVerifier, jwtService, analyticsRepo, log)
	scribeSvc := service.NewScribeService(scribeRepoAdapter, pgRepo, analyticsRepo, log)
	analyticsSvc := service.NewAnalyticsService(analyticsRepo, scribeRepoAdapter, log)

	httpHandler := httpAdapter.NewHttpHandler(authSvc, scribeSvc, analyticsSvc)
	router := httpAdapter.NewRouter(httpHandler, jwtService, cache, log, cfg.CORS.AllowedOrigins)
	engine := router.SetupEngine()

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info("http server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("shutdown signal received", zap.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced shutdown", zap.Error(err))
	}
	log.Info("server shutdown completed")
}

func connectPostgres(cfg config.PostgresConfig, log *zap.Logger) (*sql.DB, error) {
	var db *sql.DB
	var err error

	for i := 0; i < 5; i++ {
		db, err = sql.Open("postgres", cfg.DSN)
		if err == nil {
			if err = db.Ping(); err == nil {
				db.SetMaxOpenConns(cfg.MaxOpenConns)
				db.SetMaxIdleConns(cfg.MaxIdleConns)
				db.SetConnMaxLifetime(cfg.MaxLifetime)
				log.Info("postgres connection established")
				return db, nil
			}
		}
		log.Warn("postgres connection attempt failed", zap.Int("attempt", i+1), zap.Error(err))
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect to postgres after retries: %w", err)
}
