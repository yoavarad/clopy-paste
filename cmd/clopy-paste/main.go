// Command clopy-paste is the HTTP service generated from YDK component manifests.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kafkaadapter "clopy-paste/internal/adapters/kafka"
	"clopy-paste/internal/config"
	"clopy-paste/internal/httpapi"
	"clopy-paste/internal/messaging"
)

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

const (
	// listenPort is fixed; LISTEN_HOST (default all interfaces) picks the bind address.
	listenPort = "8080"
	// shutdownTimeout bounds graceful shutdown (ydk:nfr:system/shutdown-time).
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(os.Getenv("LOG_LEVEL"))}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	logger.Info("starting", "version", version)
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	kafkaClient, err := kafkaadapter.Connect()
	if err != nil {
		return err
	}
	defer kafkaClient.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	api := httpapi.NewServer(httpapi.Deps{
		Logger:       logger,
		Censor:       messaging.NewCensor(cfg.CensorChars),
		Publisher:    kafkaadapter.NewPublisher(kafkaClient),
		Store:        messaging.NewStore(),
		InstanceName: cfg.InstanceName,
	})
	listenAddr := net.JoinHostPort(os.Getenv("LISTEN_HOST"), listenPort)
	srv := &http.Server{Addr: listenAddr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", listenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutdown started")
	api.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

// parseLevel maps DEBUG/INFO/WARN/ERROR to a slog level, defaulting to INFO.
func parseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return level
}
