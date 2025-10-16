package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/config"
	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
	"github.com/redis/go-redis/v9"
)

// RedisStateManager handles state persistence using Redis
type RedisStateManager struct {
	client       *redis.Client
	keyPrefix    string
	saveInterval time.Duration
	state        State
	mu           sync.RWMutex
	dirty        bool
	stopCh       chan struct{}
	doneCh       chan struct{}
	ctx          context.Context
}

// NewRedisStateManager creates a new Redis-based state manager
func NewRedisStateManager(redisConfig config.RedisConfig, saveInterval time.Duration) (*RedisStateManager, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	m := &RedisStateManager{
		client:       client,
		keyPrefix:    redisConfig.KeyPrefix,
		saveInterval: saveInterval,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		ctx:          ctx,
	}

	// Try to load existing state
	if err := m.load(); err != nil {
		// If key doesn't exist, start fresh
		if err != redis.Nil {
			return nil, fmt.Errorf("failed to load state from Redis: %w", err)
		}
		// Initialize with zero state
		m.state = State{
			LastProcessedTimestamp: 0,
			LastProcessedFile:      "",
			TotalFilesProcessed:    0,
			TotalBytesProcessed:    0,
			LastUpdated:            time.Now().Unix(),
		}
	}

	return m, nil
}

// Start begins the periodic state persistence
func (m *RedisStateManager) Start() {
	go m.periodicSave()
}

// Stop stops the periodic persistence and saves final state
func (m *RedisStateManager) Stop() {
	close(m.stopCh)
	<-m.doneCh
	_ = m.Save() // Final save
}

// GetLastTimestamp returns the last processed timestamp
func (m *RedisStateManager) GetLastTimestamp() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LastProcessedTimestamp
}

// GetLastFile returns the last processed file path
func (m *RedisStateManager) GetLastFile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LastProcessedFile
}

// UpdateProgress updates the processing progress
func (m *RedisStateManager) UpdateProgress(timestamp int64, filePath string, bytesProcessed int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if timestamp > m.state.LastProcessedTimestamp {
		m.state.LastProcessedTimestamp = timestamp
	}
	m.state.LastProcessedFile = filePath
	m.state.TotalFilesProcessed++
	m.state.TotalBytesProcessed += bytesProcessed
	m.state.LastUpdated = time.Now().Unix()
	m.dirty = true
}

// GetStats returns current statistics
func (m *RedisStateManager) GetStats() (filesProcessed, bytesProcessed int64, lastTimestamp int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.TotalFilesProcessed, m.state.TotalBytesProcessed, m.state.LastProcessedTimestamp
}

// Save persists the current state to Redis
func (m *RedisStateManager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty {
		return nil // No changes to save
	}

	data, err := json.Marshal(m.state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	key := fmt.Sprintf("%s:state", m.keyPrefix)
	if err := m.client.Set(m.ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save state to Redis: %w", err)
	}

	m.dirty = false
	return nil
}

// load reads state from Redis
func (m *RedisStateManager) load() error {
	key := fmt.Sprintf("%s:state", m.keyPrefix)
	data, err := m.client.Get(m.ctx, key).Result()
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(data), &m.state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}

// periodicSave saves state at regular intervals
func (m *RedisStateManager) periodicSave() {
	ticker := time.NewTicker(m.saveInterval)
	defer ticker.Stop()
	defer close(m.doneCh)

	for {
		select {
		case <-ticker.C:
			if err := m.Save(); err != nil {
				// Log error but don't crash
				logging.GetDefaultLogger().Error("Failed to save state to Redis periodically", "error", err)
			}
		case <-m.stopCh:
			return
		}
	}
}

// MigrateFromFile migrates state from file-based storage to Redis
func (m *RedisStateManager) MigrateFromFile(fileManager *Manager) error {
	// Get current state from file manager
	files, bytes, timestamp := fileManager.GetStats()
	lastFile := fileManager.GetLastFile()

	// Update Redis state
	m.mu.Lock()
	m.state = State{
		LastProcessedTimestamp: timestamp,
		LastProcessedFile:      lastFile,
		TotalFilesProcessed:    files,
		TotalBytesProcessed:    bytes,
		LastUpdated:            time.Now().Unix(),
	}
	m.dirty = true
	m.mu.Unlock()

	// Save to Redis
	return m.Save()
}
