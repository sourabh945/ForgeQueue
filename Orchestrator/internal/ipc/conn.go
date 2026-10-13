package ipc

import (
	"fmt"
	"net"
	"os"
	"time"
)

const (
	DefaultAcceptWorkerTimeout = 5 * time.Second
	DefaultHeloTimeout         = 500 * time.Millisecond
)

// CreateListner create a stable socket file for connection
func CreateListener(socketPath string) (*net.UnixListener, error) {

	// if the socket file is already present from previous converstation or crashed it will clear it
	os.Remove(socketPath)

	listner, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("ipc.CreateListener: %w", err)
	}

	if err := os.Chmod(socketPath, 0600); err != nil {
		listner.Close()
		return nil, fmt.Errorf("ipc.CreateListener: chmod: %w", err)
	}

	listner.SetUnlinkOnClose(true) // auto-remove socket file on close

	return listner, nil
}

// AcceptWorker block until the worker proccess connects to the socket with timeout buildin
func AcceptWorker(listener *net.UnixListener, timeout *time.Duration) (net.Conn, error) {

	var AcceptWorkerTimeout time.Duration = DefaultAcceptWorkerTimeout

	if timeout != nil {
		AcceptWorkerTimeout = *timeout
	}

	// timeout for the accept connection
	if err := listener.SetDeadline(time.Now().Add(AcceptWorkerTimeout)); err != nil {
		return nil, fmt.Errorf("ipc.AcceptWorker: timeout: %w", err)
	}

	conn, err := listener.AcceptUnix()
	if err != nil {
		return nil, fmt.Errorf("ipc.AcceptWorker: %w", err)
	}

	listener.SetDeadline(time.Time{}) // clear for next accept
	return conn, nil
}

// WaitForHelo read the helo from the worker and confirm the worker is ready with timeout
func WaitForHelo(conn net.Conn, timeout *time.Duration) error {

	var Timeout time.Duration = DefaultHeloTimeout
	if timeout != nil {
		Timeout = *timeout
	}

	// Helo hands timeout
	if err := conn.SetReadDeadline(time.Now().Add(Timeout)); err != nil {
		conn.Close()
		return fmt.Errorf("ipc.WaitForHelo: timeout: %w", err)
	}

	raw, err := ReadFrame(conn)

	conn.SetReadDeadline(time.Time{})

	if err != nil {
		return fmt.Errorf("ipc.WaitForHelo: %w", err)
	}
	if msg := string(raw); msg != "HELO" {
		return fmt.Errorf("ipc.WaitForHelo: expected HELO got %q", msg)
	}
	return nil
}
