// Package config loads runtime configuration from environment variables.
// Source: ydk:crosscut:ops/environment
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var instanceNameRe = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)

// ErrInvalidConfig wraps every configuration error; the message names the variable, never its value.
var ErrInvalidConfig = errors.New("invalid config")

// Config is the typed runtime configuration, read once at startup.
type Config struct {
	// InstanceName is INSTANCE_NAME (required): 1..63 chars [a-z0-9-]; UI label and default consumer group suffix
	InstanceName string
	// KafkaBootstrapServers is KAFKA_BOOTSTRAP_SERVERS (required): comma-separated host:port, parsed
	KafkaBootstrapServers []string
	// KafkaInputTopic is KAFKA_INPUT_TOPIC (required): opaque topic name (ADR-001)
	KafkaInputTopic string
	// KafkaOutputTopic is KAFKA_OUTPUT_TOPIC (required): opaque topic name (ADR-001)
	KafkaOutputTopic string
	// KafkaConsumerGroup is KAFKA_CONSUMER_GROUP: consumer group id; prod supplies its own (ADR-017)
	KafkaConsumerGroup string
	// KafkaSecurityProtocol is KAFKA_SECURITY_PROTOCOL: PLAINTEXT or SASL_SSL only
	KafkaSecurityProtocol string
	// KafkaSaslUsername is KAFKA_SASL_USERNAME: secretKeyRef only
	KafkaSaslUsername string
	// KafkaSaslPassword is KAFKA_SASL_PASSWORD: secretKeyRef only; never logged
	KafkaSaslPassword string
	// KafkaCaFile is KAFKA_CA_FILE: PEM file path; chart mounts at /etc/clopy-paste/ca/ca.crt
	KafkaCaFile string
	// CensorChars is CENSOR_CHARS: each code point is one denied character; empty = none
	CensorChars string
	// LogLevel is LOG_LEVEL: DEBUG, INFO, WARN, ERROR
	LogLevel string
	// Gomemlimit is GOMEMLIMIT: Go runtime soft limit below the 128Mi container limit (ADR-017)
	Gomemlimit string
}

// Load reads every variable, applying defaults, and fails on the first missing required one.
func Load() (Config, error) {
	var cfg Config
	var err error
	if cfg.InstanceName, err = lookup("INSTANCE_NAME", "", true); err != nil {
		return Config{}, err
	}
	servers, err := lookup("KAFKA_BOOTSTRAP_SERVERS", "", true)
	if err != nil {
		return Config{}, err
	}
	if cfg.KafkaBootstrapServers, err = parseServers(servers); err != nil {
		return Config{}, err
	}
	if cfg.KafkaInputTopic, err = lookup("KAFKA_INPUT_TOPIC", "", true); err != nil {
		return Config{}, err
	}
	if cfg.KafkaOutputTopic, err = lookup("KAFKA_OUTPUT_TOPIC", "", true); err != nil {
		return Config{}, err
	}
	if cfg.KafkaConsumerGroup, err = lookup("KAFKA_CONSUMER_GROUP", "", false); err != nil {
		return Config{}, err
	}
	if cfg.KafkaSecurityProtocol, err = lookup("KAFKA_SECURITY_PROTOCOL", "SASL_SSL", false); err != nil {
		return Config{}, err
	}
	if cfg.KafkaSaslUsername, err = lookup("KAFKA_SASL_USERNAME", "", false); err != nil {
		return Config{}, err
	}
	if cfg.KafkaSaslPassword, err = lookup("KAFKA_SASL_PASSWORD", "", false); err != nil {
		return Config{}, err
	}
	if cfg.KafkaCaFile, err = lookup("KAFKA_CA_FILE", "", false); err != nil {
		return Config{}, err
	}
	if cfg.CensorChars, err = lookup("CENSOR_CHARS", "$!~", false); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = lookup("LOG_LEVEL", "INFO", false); err != nil {
		return Config{}, err
	}
	if cfg.Gomemlimit, err = lookup("GOMEMLIMIT", "100MiB", false); err != nil {
		return Config{}, err
	}
	if err := validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func invalid(name, why string) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalidConfig, name, why)
}

func parseServers(v string) ([]string, error) {
	parts := strings.Split(v, ",")
	for _, p := range parts {
		host, port, ok := strings.Cut(p, ":")
		if _, perr := strconv.ParseUint(port, 10, 16); !ok || host == "" || perr != nil {
			return nil, invalid("KAFKA_BOOTSTRAP_SERVERS", "entries must be host:port")
		}
	}
	return parts, nil
}

func validate(cfg *Config) error {
	if !instanceNameRe.MatchString(cfg.InstanceName) {
		return invalid("INSTANCE_NAME", "must be 1..63 chars of [a-z0-9-]")
	}
	switch cfg.KafkaSecurityProtocol {
	case "PLAINTEXT":
	case "SASL_SSL":
		if cfg.KafkaSaslUsername == "" {
			return invalid("KAFKA_SASL_USERNAME", "required for SASL_SSL")
		}
		if cfg.KafkaSaslPassword == "" {
			return invalid("KAFKA_SASL_PASSWORD", "required for SASL_SSL")
		}
	default:
		return invalid("KAFKA_SECURITY_PROTOCOL", "must be PLAINTEXT or SASL_SSL")
	}
	if !utf8.ValidString(cfg.CensorChars) {
		return invalid("CENSOR_CHARS", "must be valid UTF-8")
	}
	switch cfg.LogLevel {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return invalid("LOG_LEVEL", "must be DEBUG, INFO, WARN or ERROR")
	}
	if cfg.KafkaConsumerGroup == "" {
		cfg.KafkaConsumerGroup = "clopy-paste-" + cfg.InstanceName
	}
	return nil
}

// lookup distinguishes unset (default applies) from set-but-empty via os.LookupEnv.
func lookup(name, def string, required bool) (string, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		value = def
	}
	if required && value == "" {
		return "", fmt.Errorf("%w: %s: missing", ErrInvalidConfig, name)
	}
	return value, nil
}
