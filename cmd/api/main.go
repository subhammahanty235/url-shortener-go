package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/subhammahanty235/url-shortener/internal/config"
	"github.com/subhammahanty235/url-shortener/internal/handler"
	"github.com/subhammahanty235/url-shortener/internal/middleware"
	"github.com/subhammahanty235/url-shortener/internal/pkg/keygen"
	"github.com/subhammahanty235/url-shortener/internal/pkg/metrics"
	"github.com/subhammahanty235/url-shortener/internal/repository"
	"github.com/subhammahanty235/url-shortener/internal/repository/cache"
	"github.com/subhammahanty235/url-shortener/internal/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger := initLogger()
	defer logger.Sync()
	logger.Info("starting URL shortener service")
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Initialize metrics
	// Learning: Create metrics early so all components can use them
	m := metrics.NewMetrics()
	logger.Info("metrics initialized - Prometheus endpoint will be available at /metrics")

	db, err := repository.NewPostgresConnection(cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer repository.Close(db, logger)
	if err := repository.RunMigrations(db, logger); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}
	redisClient, err := cache.NewRedisClient(cfg.Redis, logger)
	if err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer cache.Close(redisClient, logger)

	keyGen, err := keygen.NewSnowflakeGenerator(keygen.Config{
		MachineID: getMachineID(),
		MinLength: cfg.URL.MinCodeLength,
		MaxLength: cfg.URL.MaxCodeLength,
	})
	if err != nil {
		logger.Fatal("failed to initialize key generator", zap.Error(err))
	}

	// Pass metrics to repositories
	// Learning: Metrics flow from top (main.go) to bottom (repositories)
	urlRepo := repository.NewPostgresURLRepository(db, m)
	cacheRepo := repository.NewRedisCacheRepository(redisClient, 24*time.Hour, m)

	// Pass metrics to service
	urlService := service.NewURLService(
		urlRepo,
		cacheRepo,
		keyGen,
		logger,
		m,
		service.URLServiceConfig{
			BaseURL:     cfg.Server.BaseURL,
			DefaultTTL:  cfg.URL.DefaultTTL,
			MaxTTL:      cfg.URL.MaxTTL,
			AllowCustom: cfg.URL.AllowCustom,
			CacheTTL:    24 * time.Hour,
		},
	)

	urlHandler := handler.NewURLHandler(urlService, logger)
	router := setupRouter(cfg, urlHandler, m, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// -----> rev todo
	go func() {
		logger.Info("server starting",
			zap.String("address", srv.Addr),
			zap.String("base_url", cfg.Server.BaseURL),
		)

		var err error
		if cfg.Server.TLSEnabled {
			err = srv.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited properly")

}

func setupRouter(
	cfg *config.Config,
	urlHandler *handler.URLHandler,
	m *metrics.Metrics,
	logger *zap.Logger,
) *gin.Engine {
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// Add middleware in the correct order
	// Learning: Order matters! Recovery -> Logging -> Metrics -> Your handlers
	router.Use(gin.Recovery()) // Panic recovery
	router.Use(middleware.MetricsMiddleware(m)) // Metrics tracking

	// Prometheus metrics endpoint
	// Learning: This exposes metrics in Prometheus format for scraping
	// Example: http://localhost:8080/metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check endpoint (no metrics needed for this)
	router.GET("/health", urlHandler.HealthCheck)

	// URL shortener endpoints
	redirectGroup := router.Group("/")
	redirectGroup.GET("/:shortCode", urlHandler.RedirectURL)

	api := router.Group("/api/v1")
	api.POST("/shorten", urlHandler.CreateURL)

	return router
}

func initLogger() *zap.Logger {
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	if os.Getenv("LOG_LEVEL") == "debug" {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.Development = true
	}

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}

	return logger
}

func getMachineID() int64 {
	// In production, this should come from environment variable or orchestrator
	// For Kubernetes, you might use the pod index from StatefulSet
	machineIDStr := os.Getenv("MACHINE_ID")
	if machineIDStr == "" {
		return 1
	}

	var machineID int64
	for _, c := range machineIDStr {
		if c >= '0' && c <= '9' {
			machineID = machineID*10 + int64(c-'0')
		}
	}

	if machineID > 1023 {
		machineID = machineID % 1024
	}

	return machineID
}
