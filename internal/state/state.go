package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/edgedelta/s3-edgedelta-streamer/internal/logging"
)

// State tracks the processing progress
type State struct {
	LastProcessedTimestamp int64  `json:"last_processed_timestamp"`
	LastProcessedFile      string `json:"last_processed_file"`
	TotalFilesProcessed    int64  `json:"total_files_processed"`
	TotalBytesProcessed    int64  `json:"total_bytes_processed"`
	LastUpdated            int64  `json:"last_updated"`
}

// StateManager interface for state persistence
type StateManager interface {
	Start()
	Stop()
	GetLastTimestamp() int64
	GetLastFile() string
	UpdateProgress(timestamp int64, filePath string, bytesProcessed int64)
	GetStats() (filesProcessed, bytesProcessed int64, lastTimestamp int64)
	Save() error
}

// Manager handles state persistence and updates
type Manager struct {
	filePath     string
	saveInterval time.Duration
	state        State
	mu           sync.RWMutex
	dirty        bool
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// NewManager creates a new state manager
func NewManager(filePath string, saveInterval time.Duration) (*Manager, error) {
	m := &Manager{
		filePath:     filePath,
		saveInterval: saveInterval,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	// Try to load existing state
	if err := m.load(); err != nil {
		// If file doesn't exist, start fresh
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load state: %w", err)
		}
		// Initialize with zero state
		m.state = State{
			LastProcessedTimestamp: 0,
			LastUpdated:            time.Now().Unix(),
		}
	}

	return m, nil
}

// Start begins the periodic state persistence
func (m *Manager) Start() {
	go m.periodicSave()
}

// Stop stops the periodic persistence and saves final state
func (m *Manager) Stop() {
	close(m.stopCh)
	<-m.doneCh
	_ = m.Save() // Final save
}

// GetLastTimestamp returns the last processed timestamp
func (m *Manager) GetLastTimestamp() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LastProcessedTimestamp
}

// GetLastFile returns the last processed file path
func (m *Manager) GetLastFile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.LastProcessedFile
}

// UpdateProgress updates the processing progress
func (m *Manager) UpdateProgress(timestamp int64, filePath string, bytesProcessed int64) {
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
func (m *Manager) GetStats() (filesProcessed, bytesProcessed int64, lastTimestamp int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.TotalFilesProcessed, m.state.TotalBytesProcessed, m.state.LastProcessedTimestamp
}

// Save persists the current state to disk
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty {
		return nil // No changes to save
	}

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temp file first, then rename (atomic operation)
	tmpPath := m.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpPath, m.filePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	m.dirty = false
	return nil
}

// load reads state from disk
func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &m.state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}

// periodicSave saves state at regular intervals
func (m *Manager) periodicSave() {
	ticker := time.NewTicker(m.saveInterval)
	defer ticker.Stop()
	defer close(m.doneCh)

	for {
		select {
		case <-ticker.C:
			if err := m.Save(); err != nil {
				// Log error but don't crash
				logging.GetDefaultLogger().Error("Failed to save state periodically", "error", err)
			}
		case <-m.stopCh:
			return
		}
	}
}
