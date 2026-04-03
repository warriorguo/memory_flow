package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/warriorguo/memory_flow/backend/internal/config"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/handler"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed successfully")

	// Initialize repositories
	projectRepo := repository.NewProjectRepo(pool)
	issueRepo := repository.NewIssueRepo(pool)
	issueHistoryRepo := repository.NewIssueHistoryRepo(pool)
	memoryRepo := repository.NewMemoryRepo(pool)
	tagRepo := repository.NewTagRepo(pool)
	depRepo := repository.NewDependencyRepo(pool)

	// Initialize services
	projectSvc := service.NewProjectService(projectRepo)
	issueSvc := service.NewIssueService(issueRepo, projectRepo, issueHistoryRepo)
	progressSvc := service.NewProgressService(issueRepo)
	memorySvc := service.NewMemoryService(memoryRepo)
	depSvc := service.NewDependencyService(depRepo, issueRepo, projectRepo)

	// Initialize handlers
	projectHandler := handler.NewProjectHandler(projectSvc)
	issueHandler := handler.NewIssueHandler(issueSvc, tagRepo)
	progressHandler := handler.NewProgressHandler(progressSvc)
	memoryHandler := handler.NewMemoryHandler(memorySvc)
	tagHandler := handler.NewTagHandler(tagRepo)
	depHandler := handler.NewDependencyHandler(depSvc)

	// Set up router
	router := handler.NewRouter(
		projectHandler,
		issueHandler,
		progressHandler,
		memoryHandler,
		tagHandler,
		depHandler,
	)

	// Start server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-done
	log.Println("server shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}

func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
