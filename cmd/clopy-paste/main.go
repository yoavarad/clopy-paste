// Command clopy-paste is the HTTP service generated from YDK component manifests.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

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
	// shutdownTimeout is the one overall shutdown deadline (ydk:nfr:system/shutdown-time).
	shutdownTimeout = 10 * time.Second
)

// consumer is the Kafka consumer loop (kafka.Consumer): Run polls until ctx is cancelled.
type consumer interface {
	Run(ctx context.Context) error
}

// kafkaClient is the part of *kgo.Client the shutdown sequence drives.
type kafkaClient interface {
	Flush(ctx context.Context) error
	CommitMarkedOffsets(ctx context.Context) error
	LeaveGroupContext(ctx context.Context) error
	Close()
}

// kafkaSide is everything built on the shared Kafka client: the Kafka boundary of the app.
type kafkaSide struct {
	publisher messaging.Publisher
	consumer  consumer
	client    kafkaClient
}

// connectFunc builds the Kafka side; it must not block on the broker being reachable (ADR-016).
type connectFunc func(cfg config.Config, store messaging.Store, logger *slog.Logger) (kafkaSide, error)

// app is the process wiring; main fills it with real values, tests with fakes at the Kafka boundary.
type app struct {
	stdout          io.Writer
	addr            string
	connect         connectFunc
	shutdownTimeout time.Duration
	onReady         func(net.Addr) // test hook; nil in production
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	code := app{
		stdout:          os.Stdout,
		addr:            net.JoinHostPort(os.Getenv("LISTEN_HOST"), listenPort),
		connect:         connectKafka,
		shutdownTimeout: shutdownTimeout,
	}.run(ctx)
	stop()
	os.Exit(code)
}

// connectKafka creates the shared franz-go client (no broker round trip) and its adapters.
func connectKafka(cfg config.Config, store messaging.Store, logger *slog.Logger) (kafkaSide, error) {
	client, err := kafkaadapter.Connect(cfg)
	if err != nil {
		return kafkaSide{}, err
	}
	return kafkaSide{
		publisher: kafkaadapter.NewPublisher(client, logger),
		consumer:  kafkaadapter.NewConsumer(client, cfg.KafkaInputTopic, store, logger),
		client:    client,
	}, nil
}

// run starts the service, serves until ctx is done (SIGTERM/SIGINT), then shuts down in order
// (ydk:crosscut:system/graceful-shutdown). It returns the process exit code.
func (a app) run(ctx context.Context) int {
	cfg, err := config.Load()
	if err != nil {
		newLogger(a.stdout, slog.LevelInfo).Error("invalid config", "error", err.Error())
		return 1
	}
	logger := newLogger(a.stdout, parseLevel(cfg.LogLevel))
	slog.SetDefault(logger)
	logger.Info("starting",
		"version", version,
		"instance", cfg.InstanceName,
		"input_topic", cfg.KafkaInputTopic,
		"output_topic", cfg.KafkaOutputTopic,
		"protocol", cfg.KafkaSecurityProtocol,
		"censor_chars", utf8.RuneCountInString(cfg.CensorChars),
	)

	store := messaging.NewStoreWithLogger(logger)
	kafka, err := a.connect(cfg, store, logger)
	if err != nil {
		logger.Error("kafka client setup failed", "error", err.Error())
		return 1
	}

	api := httpapi.NewServer(httpapi.Deps{
		Logger:       logger,
		Censor:       messaging.NewCensor(cfg.CensorChars),
		Publisher:    kafka.publisher,
		Store:        store,
		InstanceName: cfg.InstanceName,
	})
	srv := &http.Server{
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", a.addr)
	if err != nil {
		logger.Error("listen failed", "addr", a.addr, "error", err.Error())
		kafka.client.Close()
		return 1
	}

	consumerCtx, stopConsumer := context.WithCancel(context.Background())
	defer stopConsumer()
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		if err := kafka.consumer.Run(consumerCtx); err != nil {
			logger.Error("consumer stopped", "error", err.Error())
		}
	}()

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	logger.Info("ready", "addr", ln.Addr().String())
	if a.onReady != nil {
		a.onReady(ln.Addr())
	}

	code := 0
	select {
	case err := <-serveErr:
		logger.Error("http server failed", "error", err.Error())
		code = 1
	case <-ctx.Done():
	}
	if !a.shutdown(logger, api, srv, kafka, stopConsumer, consumerDone) {
		return 1
	}
	return code
}

// shutdown runs the ordered sequence under one deadline. It returns false (after a WARN naming
// the step reached) when the deadline expires.
func (a app) shutdown(logger *slog.Logger, api *httpapi.Server, srv *http.Server, kafka kafkaSide,
	stopConsumer context.CancelFunc, consumerDone <-chan struct{}) bool {
	begin := time.Now()
	logger.Info("shutdown started")
	ctx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	api.SetReady(false)
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"http", srv.Shutdown},
		{"flush", kafka.client.Flush},
		{"stop consumer", func(ctx context.Context) error {
			stopConsumer()
			select {
			case <-consumerDone:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		{"commit offsets", kafka.client.CommitMarkedOffsets},
		{"leave group", kafka.client.LeaveGroupContext},
		{"close", func(ctx context.Context) error {
			closed := make(chan struct{})
			go func() {
				kafka.client.Close()
				close(closed)
			}()
			select {
			case <-closed:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
	}
	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Warn("shutdown deadline exceeded", "step", step.name, "duration_ms", time.Since(begin).Milliseconds())
				return false
			}
			logger.Warn("shutdown step failed", "step", step.name, "error", err.Error())
		}
	}
	logger.Info("shutdown complete", "duration_ms", time.Since(begin).Milliseconds())
	return true
}

func newLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// parseLevel maps DEBUG/INFO/WARN/ERROR to a slog level, defaulting to INFO.
func parseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return level
}
