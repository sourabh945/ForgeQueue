package ipc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// WriteFrame send a length-prefixed message: 4 bytes big-endian length + payload (json/task)
// return err only
func WriteFrame(conn net.Conn, data []byte) error {
	lengthHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthHeader, uint32(len(data)))

	// Write header
	if _, err := conn.Write(lengthHeader); err != nil {
		return fmt.Errorf("ipc.WriteFrame: write header: %w", err)
	}

	// Write payload
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("ipc.WriteFrame: write payload: %w", err)
	}

	return nil
}

// ReadFrame read the length-prefixed message
// Return payload and error
func ReadFrame(conn net.Conn) ([]byte, error) {
	lengthHeader := make([]byte, 4)

	if _, err := io.ReadFull(conn, lengthHeader); err != nil {
		return nil, fmt.Errorf("ipc.ReadFrame: read header: %w", err)
	}

	msgLength := binary.BigEndian.Uint32(lengthHeader)

	payload := make([]byte, msgLength)

	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, fmt.Errorf("ipc.ReadFrame: read payload: %w", err)
	}

	return payload, nil
}
