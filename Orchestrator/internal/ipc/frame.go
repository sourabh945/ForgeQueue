package ipc

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// FrameTimeout make sure we don't hang on sending/receving the frame on socket
const FrameTimeout = 500 * time.Millisecond

// Maximum size of the payload that allowed to pass through the socket
// this is for prevent the Fake-header OOM becuase 4 bytes can go to 4 GiB of payload size
// that make the memory allocation problem
const MaxPayloadSizeInMiB = 16
const MaxPayloadSize = MaxPayloadSizeInMiB * 1024 * 1024 // 16 MiB

// WriteFrame send a length-prefixed message: 4 bytes big-endian length + payload (json/task)
// return err only
func WriteFrame(conn net.Conn, data []byte) error {

	// adding timeout for write data into the socket
	defer conn.SetWriteDeadline(time.Time{})
	if err := conn.SetWriteDeadline(time.Now().Add(FrameTimeout)); err != nil {
		return fmt.Errorf("ipc.WriteFrame: timeout: %w", err)
	}

	// checks the size of the frame is not greater than the MaxPayload
	if len(data) > MaxPayloadSize {
		return fmt.Errorf("ipc.WriteFrame: payload %.2f MiB exceeds max %d MiB", float64(len(data))/(1024*1024), MaxPayloadSizeInMiB)
	}

	// make the lengthHeader for payload
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
// Return payload and error if the payload
func ReadFrame(conn net.Conn) ([]byte, error) {

	// adding timeout for write data into the socket
	defer conn.SetReadDeadline(time.Time{})
	if err := conn.SetReadDeadline(time.Now().Add(FrameTimeout)); err != nil {
		return nil, fmt.Errorf("ipc.WriteFrame: timeout: %w", err)
	}

	lengthHeader := make([]byte, 4)

	if _, err := io.ReadFull(conn, lengthHeader); err != nil {
		return nil, fmt.Errorf("ipc.ReadFrame: read header: %w", err)
	}

	msgLength := binary.BigEndian.Uint32(lengthHeader)

	// Make sure the msgLength is not greater than allowed Maximum msgLength
	if msgLength > MaxPayloadSize {
		return nil, fmt.Errorf("ipc.ReadFrame: payload %.2f MiB exceeds max %d MiB", float64(msgLength/(1024*1024)), MaxPayloadSizeInMiB)
	}

	payload := make([]byte, msgLength)

	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, fmt.Errorf("ipc.ReadFrame: read payload: %w", err)
	}

	return payload, nil
}
