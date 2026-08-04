package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yccoskun/website/internal/config"
	"github.com/yccoskun/website/internal/database"
	"github.com/yccoskun/website/internal/handlers"
	"github.com/yccoskun/website/internal/services"
	"github.com/yccoskun/website/internal/static"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	staticDir, err := static.ResolveOverride(cfg.StaticDir, cfg.AllowStaticDir)
	if err != nil {
		log.Fatalf("static dir: %v", err)
	}
	spa, err := static.Handler(staticDir)
	if err != nil {
		log.Fatalf("static handler: %v", err)
	}
	if staticDir != "" {
		log.Printf("serving static files from %s", staticDir)
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	sessions := services.NewSessionService(db)
	if err := sessions.DestroyExpired(); err != nil {
		log.Printf("cleanup expired sessions: %v", err)
	}

	media, err := services.NewMediaService(db, cfg.UploadsDir)
	if err != nil {
		log.Fatalf("media service: %v", err)
	}
	pages := services.NewPageService(db)
	resume := services.NewResumeService(db).WithPages(pages, media)

	deps := handlers.Deps{
		DB:       db,
		Posts:    services.NewPostService(db),
		Resume:   resume,
		Sessions: sessions,
		Settings: services.NewSettingsService(db),
		Pages:    pages,
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Media:    media,
		Config:   cfg,
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handlers.NewRouter(spa, deps),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go runSessionCleanup(ctx, sessions)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		log.Fatalf("server: %v", err)
	case <-ctx.Done():
	}

	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Printf("shutdown: %v", err)
	}
}

func runSessionCleanup(ctx context.Context, sessions *services.SessionService) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sessions.DestroyExpired(); err != nil {
				log.Printf("cleanup expired sessions: %v", err)
			}
		}
	}
}
