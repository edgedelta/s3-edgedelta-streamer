package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// Test with non-existent file
	filePath := "/tmp/test_state_nonexistent.json"
	saveInterval := 30 * time.Second

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.filePath != filePath {
		t.Errorf("Expected filePath %s, got %s", filePath, manager.filePath)
	}

	if manager.saveInterval != saveInterval {
		t.Errorf("Expected saveInterval %v, got %v", saveInterval, manager.saveInterval)
	}

	// Check initial state
	if manager.state.LastProcessedTimestamp != 0 {
		t.Errorf("Expected initial LastProcessedTimestamp 0, got %d", manager.state.LastProcessedTimestamp)
	}

	if manager.state.LastProcessedFile != "" {
		t.Errorf("Expected initial LastProcessedFile empty, got %s", manager.state.LastProcessedFile)
	}
}

func TestNewManager_LoadExisting(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_state.json")

	// Create initial state file
	initialState := State{
		LastProcessedTimestamp: 1234567890,
		LastProcessedFile:      "test_file.log",
		TotalFilesProcessed:    42,
		TotalBytesProcessed:    1024,
		LastUpdated:            time.Now().Unix(),
	}

	data, err := json.MarshalIndent(initialState, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test state: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test state file: %v", err)
	}

	// Create manager that should load existing state
	saveInterval := 30 * time.Second
	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Check loaded state
	if manager.state.LastProcessedTimestamp != initialState.LastProcessedTimestamp {
		t.Errorf("Expected LastProcessedTimestamp %d, got %d", initialState.LastProcessedTimestamp, manager.state.LastProcessedTimestamp)
	}

	if manager.state.LastProcessedFile != initialState.LastProcessedFile {
		t.Errorf("Expected LastProcessedFile %s, got %s", initialState.LastProcessedFile, manager.state.LastProcessedFile)
	}

	if manager.state.TotalFilesProcessed != initialState.TotalFilesProcessed {
		t.Errorf("Expected TotalFilesProcessed %d, got %d", initialState.TotalFilesProcessed, manager.state.TotalFilesProcessed)
	}
}

func TestManager_Getters(t *testing.T) {
	filePath := "/tmp/test_getters.json"
	saveInterval := 30 * time.Second

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Test initial values
	if ts := manager.GetLastTimestamp(); ts != 0 {
		t.Errorf("Expected initial timestamp 0, got %d", ts)
	}

	if file := manager.GetLastFile(); file != "" {
		t.Errorf("Expected initial file empty, got %s", file)
	}

	// Update progress and test getters
	testTimestamp := int64(1234567890)
	testFile := "test_file.log"
	testBytes := int64(2048)

	manager.UpdateProgress(testTimestamp, testFile, testBytes)

	if ts := manager.GetLastTimestamp(); ts != testTimestamp {
		t.Errorf("Expected timestamp %d, got %d", testTimestamp, ts)
	}

	if file := manager.GetLastFile(); file != testFile {
		t.Errorf("Expected file %s, got %s", testFile, file)
	}
}

func TestManager_UpdateProgress(t *testing.T) {
	filePath := "/tmp/test_update.json"
	saveInterval := 30 * time.Second

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Update progress multiple times
	manager.UpdateProgress(1000, "file1.log", 1024)
	manager.UpdateProgress(2000, "file2.log", 2048)
	manager.UpdateProgress(1500, "file3.log", 512) // Earlier timestamp, should not update LastProcessedTimestamp

	// Check final state
	if ts := manager.GetLastTimestamp(); ts != 2000 {
		t.Errorf("Expected timestamp 2000, got %d", ts)
	}

	if file := manager.GetLastFile(); file != "file3.log" {
		t.Errorf("Expected file 'file3.log', got '%s'", file)
	}

	files, bytes, _ := manager.GetStats()
	if files != 3 {
		t.Errorf("Expected 3 files processed, got %d", files)
	}

	if bytes != 1024+2048+512 {
		t.Errorf("Expected 3584 bytes processed, got %d", bytes)
	}
}

func TestManager_GetStats(t *testing.T) {
	filePath := "/tmp/test_stats.json"
	saveInterval := 30 * time.Second

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Initial stats should be zero
	files, bytes, ts := manager.GetStats()
	if files != 0 || bytes != 0 || ts != 0 {
		t.Errorf("Expected initial stats (0,0,0), got (%d,%d,%d)", files, bytes, ts)
	}

	// Update and check stats
	manager.UpdateProgress(1000, "file1.log", 1024)
	manager.UpdateProgress(2000, "file2.log", 2048)

	files, bytes, ts = manager.GetStats()
	if files != 2 {
		t.Errorf("Expected 2 files, got %d", files)
	}

	if bytes != 3072 {
		t.Errorf("Expected 3072 bytes, got %d", bytes)
	}

	if ts != 2000 {
		t.Errorf("Expected timestamp 2000, got %d", ts)
	}
}

func TestManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_save.json")
	saveInterval := 30 * time.Second

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Update state
	manager.UpdateProgress(1234567890, "test_file.log", 1024)

	// Save
	err = manager.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load and verify content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	var loadedState State
	if err := json.Unmarshal(data, &loadedState); err != nil {
		t.Fatalf("Failed to unmarshal saved state: %v", err)
	}

	if loadedState.LastProcessedTimestamp != 1234567890 {
		t.Errorf("Expected timestamp 1234567890, got %d", loadedState.LastProcessedTimestamp)
	}

	if loadedState.LastProcessedFile != "test_file.log" {
		t.Errorf("Expected file 'test_file.log', got '%s'", loadedState.LastProcessedFile)
	}

	if loadedState.TotalFilesProcessed != 1 {
		t.Errorf("Expected 1 file processed, got %d", loadedState.TotalFilesProcessed)
	}

	if loadedState.TotalBytesProcessed != 1024 {
		t.Errorf("Expected 1024 bytes, got %d", loadedState.TotalBytesProcessed)
	}
}

func TestManager_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_startstop.json")
	saveInterval := 100 * time.Millisecond // Short interval for testing

	manager, err := NewManager(filePath, saveInterval)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Start the manager
	manager.Start()

	// Update state
	manager.UpdateProgress(1234567890, "test_file.log", 1024)

	// Wait a bit for periodic save
	time.Sleep(250 * time.Millisecond)

	// Stop the manager
	manager.Stop()

	// Verify file was saved
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("State file was not created by periodic save")
	}
}
