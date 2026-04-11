package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestParseConfig_YAMLWithDefaultInterval(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `mqtt:
  url: tcp://u:p@localhost:1883
  uniqueId: test-device
  topicPrefix: homeassistant
  name: Test

duc:
  url: http://u:p@localhost
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := parseConfig(Options{ConfigFile: configPath})
	if cfg.IntervalSeconds != 10 {
		t.Fatalf("expected default interval 10, got %d", cfg.IntervalSeconds)
	}
	if cfg.Mqtt.UniqueId != "test-device" {
		t.Fatalf("unexpected mqtt unique id: %q", cfg.Mqtt.UniqueId)
	}
}

func TestParseConfig_JSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	content := `{"mqtt":{"url":"tcp://u:p@localhost:1883","uniqueId":"json-device","topicPrefix":"ha","name":"Json Test"},"duc":{"url":"http://u:p@localhost","disallowedPrefixes":["x."]},"intervalSeconds":30}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := parseConfig(Options{ConfigFile: configPath})
	if cfg.IntervalSeconds != 30 {
		t.Fatalf("expected interval 30, got %d", cfg.IntervalSeconds)
	}
	if len(cfg.Duc.DisallowedPrefixes) != 1 || cfg.Duc.DisallowedPrefixes[0] != "x." {
		t.Fatalf("unexpected disallowed prefixes: %#v", cfg.Duc.DisallowedPrefixes)
	}
}

func TestSetLogLevel(t *testing.T) {
	original := zerolog.GlobalLevel()
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(original)
	})

	setLogLevel(0)
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Fatalf("expected info level, got %s", zerolog.GlobalLevel())
	}

	setLogLevel(1)
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Fatalf("expected debug level, got %s", zerolog.GlobalLevel())
	}

	setLogLevel(2)
	if zerolog.GlobalLevel() != zerolog.TraceLevel {
		t.Fatalf("expected trace level, got %s", zerolog.GlobalLevel())
	}
}
