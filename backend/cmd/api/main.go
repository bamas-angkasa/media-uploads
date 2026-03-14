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

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/yourusername/media-share/config"
	"github.com/yourusername/media-share/internal/admin"
	"github.com/yourusername/media-share/internal/auth"
	"github.com/yourusername/media-share/internal/database"
	"github.com/yourusername/media-share/internal/media"
	"github.com/yourusername/media-share/internal/processor"
	"github.com/yourusername/media-share/internal/public"
	"github.com/yourusername/media-share/internal/storage"
	"github.com/yourusername/media-share/internal/upload"
	migrations "github.com/yourusername/media-share/migrations"
)

func main() {
	// 1. Load config
	cfg := config.Load()

	// 2. Logger
	var logger *zap.Logger
	if cfg.App.Env == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	// 3. Context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Database pool
	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 5. Run migrations via goose + pgx stdlib adapter
	sqlDB := stdlib.OpenDBFromPool(pool)
	if err := runMigrations(sqlDB, logger); err != nil {
		logger.Fatal("migrations failed", zap.Error(err))
	}

	// 6. Redis
	rdb := database.NewRedisClient(cfg.Redis)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// 7. S3 client
	s3Client, err := storage.NewS3Client(cfg.AWS)
	if err != nil {
		logger.Fatal("failed to init S3 client", zap.Error(err))
	}

	// 8. Processor
	proc := processor.New(s3Client, pool, rdb)
	proc.Start(ctx, cfg.Media.WorkerConcurrency)

	// 9. Services
	authSvc := auth.NewService(pool, rdb, cfg.JWT)
	uploadSvc := upload.NewService(pool, rdb, s3Client, proc, cfg.Media)
	mediaSvc := media.NewService(pool, s3Client)
	publicSvc := public.NewService(pool, rdb, s3Client)
	adminSvc := admin.NewService(pool, s3Client)

	// 10. Handlers
	authHandler := auth.NewHandler(authSvc)
	uploadHandler := upload.NewHandler(uploadSvc)
	mediaHandler := media.NewHandler(mediaSvc)
	publicHandler := public.NewHandler(publicSvc)
	adminHandler := admin.NewHandler(adminSvc)

	// 11. Router
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS
	allowOrigin := os.Getenv("NEXT_PUBLIC_APP_URL")
	if cfg.App.Env == "development" {
		allowOrigin = "http://localhost:3000"
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{allowOrigin},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Routes
	api := r.Group("/api")

	// Auth — public
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.Refresh)

	// Public feed
	pub := api.Group("/public")
	pub.GET("/explore", publicHandler.Explore)
	pub.GET("/search", publicHandler.Search)
	pub.GET("/:short_code", publicHandler.GetByShortCode)
	pub.POST("/:short_code/view", publicHandler.RecordView)
	pub.POST("/:short_code/download", publicHandler.RecordDownload)

	// Report requires auth
	authedPub := api.Group("/public")
	authedPub.Use(auth.JWTMiddleware(cfg.JWT))
	authedPub.POST("/:short_code/report", publicHandler.CreateReport)

	// Authenticated routes
	authed := api.Group("")
	authed.Use(auth.JWTMiddleware(cfg.JWT))
	{
		authed.POST("/auth/logout", authHandler.Logout)
		authed.GET("/auth/me", authHandler.Me)

		authed.POST("/upload/sign", uploadHandler.Sign)
		authed.POST("/upload/confirm", uploadHandler.Confirm)
		authed.GET("/upload/progress/:id", uploadHandler.Progress)

		authed.GET("/media", mediaHandler.List)
		authed.GET("/media/:id", mediaHandler.Get)
		authed.PATCH("/media/:id", mediaHandler.Update)
		authed.DELETE("/media/:id", mediaHandler.Delete)
	}

	// Admin routes
	adminGroup := api.Group("/admin")
	adminGroup.Use(auth.JWTMiddleware(cfg.JWT), auth.RequireRole("admin"))
	{
		adminGroup.GET("/media", adminHandler.ListMedia)
		adminGroup.DELETE("/media/:id", adminHandler.DeleteMedia)
		adminGroup.GET("/users", adminHandler.ListUsers)
		adminGroup.PATCH("/users/:id", adminHandler.UpdateUser)
		adminGroup.GET("/reports", adminHandler.ListReports)
		adminGroup.PATCH("/reports/:id", adminHandler.UpdateReport)
		adminGroup.GET("/stats", adminHandler.Stats)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 12. Start view flusher
	go publicSvc.StartViewFlusher(ctx)

	// 13. HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("starting server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// 14. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}

func runMigrations(db *sql.DB, logger *zap.Logger) error {
	logger.Info("running database migrations")

	// Use embedded FS — no external files needed at runtime
	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// "." because the FS root contains the *.sql files directly
	if err := goose.Up(db, "."); err != nil {
		return err
	}

	version, err := goose.GetDBVersion(db)
	if err == nil {
		logger.Info("migrations applied", zap.Int64("version", version))
	}
	return nil
}
