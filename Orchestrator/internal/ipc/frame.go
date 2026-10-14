package ipc

import (
	"encoding/binary"
	"io"
	"net"
	"orch/internal/utils"
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

	// creating error handler
	errh := utils.ErrorHandler{
		Pkg: "ipc",
		Fxn: "WriteFrame",
	}

	// adding timeout for write data into the socket
	defer conn.SetWriteDeadline(time.Time{})
	if err := conn.SetWriteDeadline(time.Now().Add(FrameTimeout)); err != nil {
		return errh.Errorf("timeout: %w", err)
	}

	// checks the size of the frame is not greater than the MaxPayload
	if len(data) > MaxPayloadSize {
		return errh.Errorf("payload %.2f MiB exceeds max %d MiB", float64(len(data))/(1024*1024), MaxPayloadSizeInMiB)
	}

	// make the lengthHeader for payload
	lengthHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthHeader, uint32(len(data)))

	// Write header
	if _, err := conn.Write(lengthHeader); err != nil {
		return errh.Errorf("write header: %w", err)
	}

	// Write payload
	if _, err := conn.Write(data); err != nil {
		return errh.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadFrame read the length-prefixed message
// Return payload and error if the payload
func ReadFrame(conn net.Conn) ([]byte, error) {

	// creating error handler
	errh := utils.ErrorHandler{
		Pkg: "ipc",
		Fxn: "ReadFrame",
	}

	// adding timeout for write data into the socket
	defer conn.SetReadDeadline(time.Time{})
	if err := conn.SetReadDeadline(time.Now().Add(FrameTimeout)); err != nil {
		return nil, errh.Errorf("timeout: %w", err)
	}

	lengthHeader := make([]byte, 4)

	if _, err := io.ReadFull(conn, lengthHeader); err != nil {
		return nil, errh.Errorf("read header: %w", err)
	}

	msgLength := binary.BigEndian.Uint32(lengthHeader)

	// Make sure the msgLength is not greater than allowed Maximum msgLength
	if msgLength > MaxPayloadSize {
		return nil, errh.Errorf("payload %.2f MiB exceeds max %d MiB", float64(msgLength)/(1024*1024), MaxPayloadSizeInMiB)
	}

	payload := make([]byte, msgLength)

	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, errh.Errorf("read payload: %w", err)
	}

	return payload, nil
}
