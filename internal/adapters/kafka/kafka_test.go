package kafka

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"

	"clopy-paste/internal/config"
	"clopy-paste/internal/messaging"
)

func baseConfig() config.Config {
	return config.Config{
		KafkaBootstrapServers: []string{"127.0.0.1:1"},
		KafkaInputTopic:       "in-topic",
		KafkaOutputTopic:      "out-topic",
		KafkaConsumerGroup:    "clopy-paste-a",
		KafkaSecurityProtocol: "PLAINTEXT",
	}
}

// newTestClient builds a client from cfg without contacting a broker (seed is unreachable loopback).
func newTestClient(t *testing.T, cfg config.Config) *kgo.Client {
	t.Helper()
	cl, err := Connect(cfg)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(cl.Close)
	return cl
}

func writeCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConnectCommonOptions(t *testing.T) {
	cl := newTestClient(t, baseConfig())

	if got := cl.OptValue(kgo.SeedBrokers).([]string); !slices.Equal(got, []string{"127.0.0.1:1"}) {
		t.Errorf("seeds = %v", got)
	}
	if got := cl.OptValue(kgo.DefaultProduceTopic); got != "out-topic" {
		t.Errorf("default produce topic = %v", got)
	}
	if got := cl.OptValue(kgo.RequiredAcks); got != kgo.AllISRAcks() {
		t.Errorf("acks = %v, want all", got)
	}
	if got := cl.OptValue(kgo.DisableIdempotentWrite); got != false {
		t.Errorf("idempotence disabled = %v", got)
	}
	if got := cl.OptValue(kgo.ConsumerGroup); got != "clopy-paste-a" {
		t.Errorf("group = %v", got)
	}
	if topics := cl.OptValue(kgo.ConsumeTopics).(map[string]*regexp.Regexp); len(topics) != 0 {
		t.Errorf("consume topics = %v, want none (Consumer.Run subscribes)", topics)
	}
	if got := cl.OptValue(kgo.ConsumeResetOffset); got != kgo.NewOffset().AtEnd() {
		t.Errorf("reset offset = %v, want end", got)
	}
	if got := cl.OptValue(kgo.AutoCommitMarks); got != true {
		t.Errorf("auto commit marks = %v", got)
	}
	if got := cl.OptValue(kgo.AutoCommitInterval); got != 5*time.Second {
		t.Errorf("auto commit interval = %v", got)
	}
}

func TestConnectPlaintextHasNoTLSOrSASL(t *testing.T) {
	cl := newTestClient(t, baseConfig())

	if got := cl.OptValue(kgo.DialTLSConfig).(*tls.Config); got != nil {
		t.Errorf("TLS config = %v, want nil", got)
	}
	if got := cl.OptValue(kgo.SASL).([]sasl.Mechanism); len(got) != 0 {
		t.Errorf("SASL mechanisms = %v, want none", got)
	}
}

func saslConfig(caFile string) config.Config {
	cfg := baseConfig()
	cfg.KafkaSecurityProtocol = "SASL_SSL"
	cfg.KafkaSaslUsername = "svc-user"
	cfg.KafkaSaslPassword = "s3cret"
	cfg.KafkaCaFile = caFile
	return cfg
}

func TestConnectSASLSSLUsesScramSha512AndCAFile(t *testing.T) {
	cl := newTestClient(t, saslConfig(writeCA(t)))

	tlsCfg := cl.OptValue(kgo.DialTLSConfig).(*tls.Config)
	if tlsCfg == nil {
		t.Fatal("TLS config is nil")
	}
	if tlsCfg.RootCAs == nil {
		t.Error("RootCAs not set from KAFKA_CA_FILE")
	}
	if tlsCfg.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %x", tlsCfg.MinVersion)
	}
	mechs := cl.OptValue(kgo.SASL).([]sasl.Mechanism)
	if len(mechs) != 1 || mechs[0].Name() != "SCRAM-SHA-512" {
		t.Errorf("SASL mechanisms = %v, want one SCRAM-SHA-512", mechs)
	}
}

func TestConnectSASLSSLWithoutCAFileUsesSystemRoots(t *testing.T) {
	cl := newTestClient(t, saslConfig(""))

	tlsCfg := cl.OptValue(kgo.DialTLSConfig).(*tls.Config)
	if tlsCfg == nil {
		t.Fatal("TLS config is nil")
	}
	if tlsCfg.RootCAs != nil {
		t.Error("RootCAs set; want nil so the system roots are used")
	}
}

func TestConnectCAFileErrors(t *testing.T) {
	invalid := filepath.Join(t.TempDir(), "bad.crt")
	if err := os.WriteFile(invalid, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"missing": filepath.Join(t.TempDir(), "absent.crt"),
		"invalid": invalid,
	} {
		t.Run(name, func(t *testing.T) {
			cl, err := Connect(saslConfig(path))
			if err == nil {
				cl.Close()
				t.Fatal("Connect succeeded, want CA file error")
			}
			if !strings.Contains(err.Error(), "KAFKA_CA_FILE") {
				t.Errorf("error %q does not name KAFKA_CA_FILE", err)
			}
			if strings.Contains(err.Error(), "s3cret") {
				t.Errorf("error leaks the password: %q", err)
			}
		})
	}
}

func TestConnectRejectsUnknownProtocol(t *testing.T) {
	cfg := baseConfig()
	cfg.KafkaSecurityProtocol = "SSL"
	if cl, err := Connect(cfg); err == nil {
		cl.Close()
		t.Fatal("Connect succeeded for SSL, want error")
	}
}

func TestNewRecord(t *testing.T) {
	rec := newRecord("alice", "héllo")

	if string(rec.Key) != "alice" {
		t.Errorf("key = %q", rec.Key)
	}
	if want := messaging.EncodeRecordValue("alice", "héllo"); string(rec.Value) != string(want) {
		t.Errorf("value = %s, want %s", rec.Value, want)
	}
	if len(rec.Headers) != 0 {
		t.Errorf("headers = %v, want none", rec.Headers)
	}
	if rec.Topic != "" {
		t.Errorf("topic = %q, want empty so DefaultProduceTopic applies", rec.Topic)
	}
}

func TestPublishMapsFailureToKafkaUnavailable(t *testing.T) {
	cl := newTestClient(t, baseConfig())
	pub := NewPublisher(cl, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := pub.Publish(ctx, "alice", "hi")

	if !errors.Is(err, messaging.ErrKafkaUnavailable) {
		t.Fatalf("err = %v, want ErrKafkaUnavailable", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the deadline cause kept", err)
	}
}

func TestPublishCapsDeadlineAtFiveSeconds(t *testing.T) {
	if publishTimeout != 5*time.Second {
		t.Errorf("publishTimeout = %v, want 5s (ADR-017)", publishTimeout)
	}
}
