package ipc

import (
	"encoding/json"
	"log/slog"
	"net"

	// Alias the import to 'typesipc' to avoid colliding with 'package ipc'
	typesipc "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

// initConnection initializes a connection to the unix socket and returns it.
// NOTE: The caller is responsible for calling conn.Close() when done.
func initConnection(socketPath string, logger *slog.Logger) net.Conn {

	// connecting to the unix socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		logger.Error("Failed to connect to socket", slog.String("error", err.Error()))
		return nil
	}

	return conn
}

// initJob sends the job over the connection and returns the connection.
func initJob(conn net.Conn, job typesipc.Job, logger *slog.Logger) net.Conn {

	// making data into json format to send
	jsonData, err := json.Marshal(job)
	if err != nil {
		logger.Error("Failed to marshal JSON", slog.String("error", err.Error()))
		conn.Close() // Clean up the connection we just opened
		return nil
	}

	// writing to socket
	_, err = conn.Write(jsonData)
	if err != nil {
		logger.Error("Failed to write to socket", slog.String("error", err.Error()))
		conn.Close()
		return nil
	}

	return conn
}

// waitForJobResponse waits for a job response from the socket and returns it.
// It closes the connection at the end.
func waitForJobResponse(conn net.Conn, logger *slog.Logger) typesipc.JobResponse {

	// closing the connection at end
	defer conn.Close()

	// reading from socket
	decoder := json.NewDecoder(conn)
	var response typesipc.JobResponse
	if err := decoder.Decode(&response); err != nil {
		logger.Error("Failed to decode response", slog.String("error", err.Error()))
		return typesipc.JobResponse{}
	}
	return response
}
