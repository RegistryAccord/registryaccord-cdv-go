// cmd/cdvd/main.go
// Package main implements the entry point for the CDV service.
// It initializes all components and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RegistryAccord/registryaccord-cdv-go/internal/config"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/event"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/identity"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/server"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/storage"
	"github.com/RegistryAccord/registryaccord-cdv-go/internal/telemetry"
)

// main is the entry point for the CDV service.
// It initializes all components, starts the HTTP server, and handles graceful shutdown.
func main() {
	// Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	// Configure structured logging for the application
	logLevel := slog.LevelInfo
	if cfg.Env == "dev" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Initialize OpenTelemetry
	_, err = telemetry.InitTracer("cdv-service")
	if err != nil {
		logger.Error("failed to initialize OpenTelemetry tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		// Shutdown the tracer provider
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		telemetry.ShutdownTracer(ctx)
	}()

	// Initialize storage backend (PostgreSQL or in-memory)
	var store storage.Store
	if cfg.DatabaseDSN != "" {
		// Use PostgreSQL storage for production
		store, err = storage.NewPostgres(cfg.DatabaseDSN)
		if err != nil {
			logger.Error("failed to initialize postgres storage", "error", err)
			os.Exit(1)
		}
	} else {
		// Use in-memory storage for development/testing
		store = storage.NewMemory()
	}

	// Initialize event publisher (NATS JetStream or no-op)
	pub := event.NewPublisherFromEnv()
	defer pub.Close() // Ensure publisher is closed on exit

	// Initialize identity client for DID validation
	var idClient *identity.Client
	if cfg.IdentityURL != "" {
		idClient = identity.New(cfg.IdentityURL)
	}

	// Create HTTP mux with all handlers and middleware
	mux := server.NewMux(store, pub, idClient, cfg.JWTIssuer, cfg.JWTAudience, cfg.MaxMediaSize, cfg.AllowedMimeTypes, nil, cfg.SpecsURL, cfg.RejectDeprecatedSchemas)

	// Create HTTP server with timeout configuration
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,           // Server address
		Handler:      mux,            // Request handler
		ReadTimeout:  5 * time.Second, // Read timeout
		WriteTimeout: 10 * time.Second, // Write timeout
	}

	// Start server in a separate goroutine
	go func() {
		logger.Info("server starting", "addr", addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Handle graceful shutdown
	logger.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	// Close PostgreSQL storage if used
	if postgresStore, ok := store.(interface{ Close() }); ok {
		postgresStore.Close()
	}

	// Note: pub.Close() is deferred above
	logger.Info("server exited")
}
