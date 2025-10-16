package tcppool

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// GetHost returns the host
func (p *Pool) GetHost() string {
	return p.host
}

// GetPort returns the port
func (p *Pool) GetPort() int {
	return p.port
}

// Pool manages a pool of TCP connections
type Pool struct {
	host   string
	port   int
	size   int
	conns  chan net.Conn
	mu     sync.Mutex
	closed bool
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewPool creates a new TCP connection pool
func NewPool(host string, port int, size int) (*Pool, error) {
	p := &Pool{
		host:   host,
		port:   port,
		size:   size,
		conns:  make(chan net.Conn, size),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Pre-create connections
	for i := 0; i < size; i++ {
		conn, err := p.createConnection()
		if err != nil {
			// Close any connections we've created so far
			close(p.conns)
			for conn := range p.conns {
				conn.Close()
			}
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}
		p.conns <- conn
	}

	// Start connection health checker
	go p.healthChecker()

	return p, nil
}

// Get retrieves a connection from the pool
func (p *Pool) Get() (net.Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}
	p.mu.Unlock()

	// Try to get a connection with timeout
	select {
	case conn := <-p.conns:
		// Test if connection is still alive
		if !p.isConnAlive(conn) {
			// Close the dead connection before creating a new one
			conn.Close()
			// Try to create a new one
			newConn, err := p.createConnection()
			if err != nil {
				return nil, fmt.Errorf("failed to create new connection: %w", err)
			}
			return newConn, nil
		}
		return conn, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for connection from pool")
	}
}

// Put returns a connection to the pool
func (p *Pool) Put(conn net.Conn) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		conn.Close()
		return
	}
	p.mu.Unlock()

	// Check if connection is still alive
	if !p.isConnAlive(conn) {
		conn.Close()
		// Try to create a replacement
		if newConn, err := p.createConnection(); err == nil {
			select {
			case p.conns <- newConn:
			default:
				// Pool is full, close the connection
				newConn.Close()
			}
		}
		return
	}

	// Return to pool
	select {
	case p.conns <- conn:
	default:
		// Pool is full, close the connection
		conn.Close()
	}
}

// Close closes all connections in the pool
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Stop health checker
	close(p.stopCh)
	<-p.doneCh

	// Close the channel
	close(p.conns)

	// Close all connections
	for conn := range p.conns {
		conn.Close()
	}

	return nil
}

// createConnection creates a new TCP connection
func (p *Pool) createConnection() (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Set TCP keepalive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			return nil, fmt.Errorf("failed to set keepalive: %w", err)
		}
		if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
			return nil, fmt.Errorf("failed to set keepalive period: %w", err)
		}
	}

	return conn, nil
}

// isConnAlive checks if a connection is still alive
func (p *Pool) isConnAlive(conn net.Conn) bool {
	// Set a short read deadline to test connection
	if err := conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond)); err != nil {
		return false
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{}) // Clear deadline, ignore error on cleanup
	}()

	// Try to read one byte
	one := make([]byte, 1)
	_, err := conn.Read(one)

	// If we get a timeout, connection is alive but no data
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Any other error means connection is dead
	return err == nil
}

// healthChecker periodically checks and replaces dead connections
func (p *Pool) healthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer close(p.doneCh)

	for {
		select {
		case <-ticker.C:
			// Check each connection in the pool
			for i := 0; i < p.size; i++ {
				select {
				case conn := <-p.conns:
					if !p.isConnAlive(conn) {
						conn.Close()
						// Create new connection
						if newConn, err := p.createConnection(); err == nil {
							p.conns <- newConn
						} else {
							// If we can't create a new connection, put back the old one
							// It will be detected as dead on next use
							p.conns <- conn
						}
					} else {
						// Connection is alive, put it back
						p.conns <- conn
					}
				default:
					// No connections available right now, skip
				}
			}
		case <-p.stopCh:
			return
		}
	}
}
