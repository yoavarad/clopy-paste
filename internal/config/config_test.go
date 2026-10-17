package config

import (
	"errors"
	"os"
	"strings"
	"testing"
)

var allVars = []string{
	"INSTANCE_NAME", "KAFKA_BOOTSTRAP_SERVERS", "KAFKA_INPUT_TOPIC", "KAFKA_OUTPUT_TOPIC",
	"KAFKA_CONSUMER_GROUP", "KAFKA_SECURITY_PROTOCOL", "KAFKA_SASL_USERNAME",
	"KAFKA_SASL_PASSWORD", "KAFKA_CA_FILE", "CENSOR_CHARS", "LOG_LEVEL", "GOMEMLIMIT",
}

// setEnv clears all vars (restored by t.Setenv cleanup is not possible for unset, so set then unset) and applies overrides.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range allVars {
		t.Setenv(k, "")
		unsetenv(t, k)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func minimal() map[string]string {
	return map[string]string{
		"INSTANCE_NAME":           "alpha",
		"KAFKA_BOOTSTRAP_SERVERS": "k1:9092,k2:9092",
		"KAFKA_INPUT_TOPIC":       "in",
		"KAFKA_OUTPUT_TOPIC":      "out",
		"KAFKA_SECURITY_PROTOCOL": "PLAINTEXT",
	}
}

func with(m map[string]string, k, v string) map[string]string {
	c := map[string]string{}
	for a, b := range m {
		c[a] = b
	}
	c[k] = v
	return c
}

func without(m map[string]string, k string) map[string]string {
	c := with(m, k, "")
	delete(c, k)
	return c
}

func TestInvalidConfig(t *testing.T) {
	t.Run("minimal-plaintext", func(t *testing.T) {
		setEnv(t, minimal())
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CensorChars != "$!~" || cfg.LogLevel != "INFO" {
			t.Fatalf("defaults: %q %q", cfg.CensorChars, cfg.LogLevel)
		}
		if len(cfg.KafkaBootstrapServers) != 2 || cfg.KafkaBootstrapServers[1] != "k2:9092" {
			t.Fatalf("servers: %v", cfg.KafkaBootstrapServers)
		}
	})
	t.Run("missing-required", func(t *testing.T) {
		for _, name := range []string{"INSTANCE_NAME", "KAFKA_BOOTSTRAP_SERVERS", "KAFKA_INPUT_TOPIC", "KAFKA_OUTPUT_TOPIC"} {
			setEnv(t, without(minimal(), name))
			_, err := Load()
			if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s: %v", name, err)
			}
		}
	})
	t.Run("bad-protocol", func(t *testing.T) {
		setEnv(t, with(minimal(), "KAFKA_SECURITY_PROTOCOL", "SSL"))
		_, err := Load()
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "KAFKA_SECURITY_PROTOCOL") {
			t.Fatal(err)
		}
		if strings.Contains(err.Error(), "SSL") && strings.Contains(strings.ReplaceAll(err.Error(), "SASL_SSL", ""), "SSL") {
			t.Fatalf("leaks value: %v", err)
		}
	})
	t.Run("sasl-needs-creds", func(t *testing.T) {
		env := without(minimal(), "KAFKA_SECURITY_PROTOCOL")
		env["KAFKA_SASL_PASSWORD"] = "s3cr3t-pw"
		setEnv(t, env)
		_, err := Load()
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "KAFKA_SASL_USERNAME") {
			t.Fatal(err)
		}
		if strings.Contains(err.Error(), "s3cr3t-pw") {
			t.Fatal("password leaked")
		}
		env = without(env, "KAFKA_SASL_PASSWORD")
		env["KAFKA_SASL_USERNAME"] = "bob"
		setEnv(t, env)
		_, err = Load()
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "KAFKA_SASL_PASSWORD") {
			t.Fatal(err)
		}
		env["KAFKA_SASL_PASSWORD"] = "pw"
		setEnv(t, env)
		if _, err = Load(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty-censor", func(t *testing.T) {
		setEnv(t, with(minimal(), "CENSOR_CHARS", ""))
		cfg, err := Load()
		if err != nil || cfg.CensorChars != "" {
			t.Fatalf("%q %v", cfg.CensorChars, err)
		}
	})
}

func TestInstanceName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"a", true}, {"abc-123", true}, {strings.Repeat("a", 63), true},
		{strings.Repeat("a", 64), false}, {"Upper", false}, {"under_score", false}, {"sp ace", false},
	}
	for _, c := range cases {
		setEnv(t, with(minimal(), "INSTANCE_NAME", c.name))
		_, err := Load()
		if c.ok && err != nil {
			t.Errorf("%q: %v", c.name, err)
		}
		if !c.ok {
			if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "INSTANCE_NAME") {
				t.Errorf("%q: %v", c.name, err)
			} else if strings.Contains(err.Error(), c.name) {
				t.Errorf("%q leaked", c.name)
			}
		}
	}
}

func TestLogLevel(t *testing.T) {
	for _, l := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		setEnv(t, with(minimal(), "LOG_LEVEL", l))
		if cfg, err := Load(); err != nil || cfg.LogLevel != l {
			t.Errorf("%s: %v", l, err)
		}
	}
	setEnv(t, with(minimal(), "LOG_LEVEL", "TRACE"))
	_, err := Load()
	if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "LOG_LEVEL") || strings.Contains(err.Error(), "TRACE") {
		t.Fatal(err)
	}
}

func TestConsumerGroup(t *testing.T) {
	setEnv(t, minimal())
	cfg, err := Load()
	if err != nil || cfg.KafkaConsumerGroup != "clopy-paste-alpha" {
		t.Fatalf("%q %v", cfg.KafkaConsumerGroup, err)
	}
	setEnv(t, with(minimal(), "KAFKA_CONSUMER_GROUP", "custom"))
	cfg, err = Load()
	if err != nil || cfg.KafkaConsumerGroup != "custom" {
		t.Fatalf("%q %v", cfg.KafkaConsumerGroup, err)
	}
}

func TestBootstrapServers(t *testing.T) {
	for _, bad := range []string{",", "nohost", "a:1,,b:2", "host:", ":9092", "host:abc", "a:1, b:2", "h:0"} {
		setEnv(t, with(minimal(), "KAFKA_BOOTSTRAP_SERVERS", bad))
		_, err := Load()
		if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "KAFKA_BOOTSTRAP_SERVERS") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func unsetenv(t *testing.T, k string) {
	t.Helper()
	if err := os.Unsetenv(k); err != nil {
		t.Fatal(err)
	}
}

func TestCensorChars(t *testing.T) {
	setEnv(t, minimal())
	if cfg, err := Load(); err != nil || cfg.CensorChars != "$!~" {
		t.Fatalf("unset: %q %v", cfg.CensorChars, err)
	}
	// Windows env cannot carry invalid UTF-8, so exercise validate directly.
	cfg := Config{InstanceName: "a", KafkaSecurityProtocol: "PLAINTEXT", LogLevel: "INFO", CensorChars: "\xff\xfe"}
	err := validate(&cfg)
	if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), "CENSOR_CHARS") {
		t.Fatalf("invalid utf8: %v", err)
	}
}
