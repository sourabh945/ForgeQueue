package ipc

import (
	"fmt"
	"net"
	"os"
)

// CreateListner create a stable socket file for connection
func CreateListener(socketPath string) (net.Listener, error) {

	// if the socket file is already present from previous converstation or crashed it will clear it
	os.Remove(socketPath)

	listner, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("ipc.CreateListener: %w", err)
	}
	return listner, nil
}

// AccpetWorker block until the worker proccess connects to the socket
func AccpetWorker(listener net.Listener) (net.Conn, error) {
	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("ipc.AcceptWorker: %w", err)
	}
	return conn, nil
}

// WaitForHelo read the helo from the worker and confirm the worker is ready
func WaitForHelo(conn net.Conn) error {
	raw, err := ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("ipc.WaitForHelo: %w", err)
	}
	if msg := string(raw); msg != "HELO" {
		return fmt.Errorf("ipc.WaitForHelo: expected HELO got %q", msg)
	}
	return nil
}
