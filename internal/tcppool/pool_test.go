package tcppool

import (
	"net"
	"testing"
	"time"
)

func TestNewPool_InvalidHost(t *testing.T) {
	// Try to create pool with invalid host
	_, err := NewPool("invalid-host-that-does-not-exist", 12345, 5)
	if err == nil {
		t.Error("Expected error for invalid host")
	}
}

func TestPool_GetHost(t *testing.T) {
	// Create a pool that will fail, but we can still test getters
	pool := &Pool{
		host: "test-host",
		port: 8080,
	}

	if host := pool.GetHost(); host != "test-host" {
		t.Errorf("Expected host 'test-host', got '%s'", host)
	}
}

func TestPool_GetPort(t *testing.T) {
	pool := &Pool{
		host: "test-host",
		port: 8080,
	}

	if port := pool.GetPort(); port != 8080 {
		t.Errorf("Expected port 8080, got %d", port)
	}
}

func TestPool_Close(t *testing.T) {
	// Create a pool with minimal initialization
	pool := &Pool{
		conns:  make(chan net.Conn, 1),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Start a goroutine to simulate the health checker
	go func() {
		<-pool.stopCh
		close(pool.doneCh)
	}()

	// Close should not panic
	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestPool_Get_ClosedPool(t *testing.T) {
	pool := &Pool{
		closed: true,
	}

	_, err := pool.Get()
	if err == nil {
		t.Error("Expected error when getting from closed pool")
	}
}

func TestPool_Put_ClosedPool(t *testing.T) {
	pool := &Pool{
		closed: true,
		conns:  make(chan net.Conn, 1),
	}

	// Create a mock connection (doesn't need to be real for this test)
	mockConn := &mockConn{}

	// Put should not panic, should close the connection
	pool.Put(mockConn)

	// Check that Close was called on the mock
	if !mockConn.closed {
		t.Error("Connection should have been closed")
	}
}

// mockConn implements net.Conn for testing
type mockConn struct {
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
