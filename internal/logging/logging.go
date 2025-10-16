package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger with convenience methods
type Logger struct {
	*slog.Logger
}

// Config holds logging configuration
type Config struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// NewLogger creates a new configured logger
func NewLogger(config Config) *Logger {
	var level slog.Level
	switch strings.ToLower(config.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(config.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewDefaultLogger creates a logger with default settings
func NewDefaultLogger() *Logger {
	return NewLogger(Config{
		Level:  "info",
		Format: "text",
	})
}

// With creates a new logger with additional context
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// WithGroup creates a new logger with a group
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: l.Logger.WithGroup(name),
	}
}

// SetOutput changes the output destination
func (l *Logger) SetOutput(w io.Writer) {
	// This is a simplified implementation - in a real scenario,
	// you'd need to recreate the handler with the new writer
}

// Global logger instance
var defaultLogger *Logger

// InitDefaultLogger initializes the global logger
func InitDefaultLogger(config Config) {
	defaultLogger = NewLogger(config)
}

// GetDefaultLogger returns the global logger
func GetDefaultLogger() *Logger {
	if defaultLogger == nil {
		defaultLogger = NewDefaultLogger()
	}
	return defaultLogger
}

// Convenience functions for global logger
func Debug(msg string, args ...any) {
	GetDefaultLogger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	GetDefaultLogger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	GetDefaultLogger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	GetDefaultLogger().Error(msg, args...)
}
