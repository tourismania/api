// Package main is the HTTP server entrypoint.
//
//	@title                      Tourismania API
//	@version                    1.0.2
//	@description                REST API for user management with JWT auth and Kafka events.
//	@securityDefinitions.apikey Bearer
//	@in                         header
//	@name                       Authorization
//	@description                Type 'Bearer ' followed by a space and then your token.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api/config"
	_ "api/docs/swagger"
	apihttp "api/internal/presentation/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server fatal", "err", err)
		os.Exit(1)
	}
}

// run owns the lifecycle so main stays one-shot — the only place
// os.Exit is allowed (per project policy).
func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container, err := config.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer container.Close()

	httpServer := apihttp.Server{
		Login:              container.Http.Login,
		CreateUser:         container.Http.CreateUser,
		GetMe:              container.Http.GetMe,
		Airports:           container.Http.Airports,
		CreateOffer:        container.Http.CreateOffer,
		GetOffer:           container.Http.GetOffer,
		GetOffers:          container.Http.GetOffers,
		GetPublishedOffer:  container.Http.GetPublishedOffer,
		UpdateOffer:        container.Http.UpdateOffer,
		DeleteOffer:        container.Http.DeleteOffer,
		JWT:                container.JWT,
		CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins,
		RateLimit:          cfg.RateLimit.RequestsPerMinute,
	}

	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           httpServer.Build(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http: listening", "addr", cfg.Server.Address, "env", cfg.App.Env, "version", cfg.App.Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown: signal received")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
