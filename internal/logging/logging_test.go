package logging

import (
	"bytes"
	"testing"
)

func TestNewLogger_TextFormat(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "text",
	}

	logger := NewLogger(config)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	if logger.Logger == nil {
		t.Error("Logger.Logger is nil")
	}
}

func TestNewLogger_JSONFormat(t *testing.T) {
	config := Config{
		Level:  "debug",
		Format: "json",
	}

	logger := NewLogger(config)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	if logger.Logger == nil {
		t.Error("Logger.Logger is nil")
	}
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	config := Config{
		Level:  "invalid",
		Format: "text",
	}

	logger := NewLogger(config)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	// Should default to info level
}

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	if logger == nil {
		t.Fatal("NewDefaultLogger returned nil")
	}

	if logger.Logger == nil {
		t.Error("Default logger.Logger is nil")
	}
}

func TestGetDefaultLogger(t *testing.T) {
	// Reset global logger
	defaultLogger = nil

	logger := GetDefaultLogger()
	if logger == nil {
		t.Fatal("GetDefaultLogger returned nil")
	}

	// Should return the same instance
	logger2 := GetDefaultLogger()
	if logger != logger2 {
		t.Error("GetDefaultLogger should return the same instance")
	}
}

func TestInitDefaultLogger(t *testing.T) {
	config := Config{
		Level:  "warn",
		Format: "json",
	}

	InitDefaultLogger(config)

	logger := GetDefaultLogger()
	if logger == nil {
		t.Fatal("GetDefaultLogger returned nil after InitDefaultLogger")
	}
}

func TestLogger_With(t *testing.T) {
	logger := NewDefaultLogger()
	childLogger := logger.With("key", "value")

	if childLogger == nil {
		t.Fatal("With returned nil")
	}

	if childLogger == logger {
		t.Error("With should return a new logger instance")
	}
}

func TestLogger_WithGroup(t *testing.T) {
	logger := NewDefaultLogger()
	groupLogger := logger.WithGroup("testgroup")

	if groupLogger == nil {
		t.Fatal("WithGroup returned nil")
	}

	if groupLogger == logger {
		t.Error("WithGroup should return a new logger instance")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Reset global logger to default
	defaultLogger = nil

	// These should not panic
	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")
}

func TestSetOutput(t *testing.T) {
	logger := NewDefaultLogger()

	// Create a buffer to capture output
	var buf bytes.Buffer

	// This is a no-op in the current implementation, but should not panic
	logger.SetOutput(&buf)
}
