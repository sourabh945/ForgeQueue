package ipc

import (
	"encoding/json"
	"log/slog"
	"net"
	"time"

	// Alias the import to 'types'
	types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

type Worker struct {
	*types.Worker
}

// InitConnection initializes a connection to the unix socket and returns it.
// NOTE: The caller is responsible for calling conn.Close() when done.
func InitConnection(socketPath string, _logger *slog.Logger) (net.Conn, error) {

	logger := _logger.With(slog.String("type", "module"), slog.String("module", "ipc.initConnection"))

	logger.Info("Connecting to socket", slog.String("socketPath", socketPath))

	var conn net.Conn
	var err error

	// connecting to the unix socket
	for range 10 {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			logger.Info("Connected to socket", slog.String("socketPath", socketPath))
			return conn, nil
		}

		// sleep for 10 seconds
		time.Sleep(100 * time.Microsecond)
	}

	logger.Error("Failed to connect to socket", slog.String("error", err.Error()))

	return nil, err
}

// InitJob sends the job over the connection
func (worker *Worker) InitJob(job types.Job) bool {

	//read lock to ensure thread safety
	worker.mu.RLock()
	defer worker.mu.RUnLock() // unlock the read lock
	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.initJob"), slog.String("jobId", job.JobId))
	conn := worker.Conn

	logger.Info("Sending job...")

	// making data into json format to send
	jsonData, err := json.Marshal(job)
	if err != nil {
		logger.Error("Failed to marshal JSON", slog.String("error", err.Error()))
		conn.Close() // Clean up the connection we just opened
		logger.Error("Failed to send job", slog.String("error", err.Error()))
		return false
	}

	// writing to socket
	_, err = conn.Write(jsonData)
	if err != nil {
		logger.Error("Failed to write to socket", slog.String("error", err.Error()))
		conn.Close()
		logger.Error("Failed to send job", slog.String("error", err.Error()))
		return false
	}

	logger.Info("Job sent successfully")
	return true

}

// WaitForJobResponse waits for a job response from the socket and returns it.
func (worker *Worker) WaitForJobResponse() (types.JobResponse, error) {

	// adding Read lock on the workef
	worker.mu.RLock()
	defer worker.mu.RUnlock()

	conn := worker.Conn
	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.waitForJobResponse"), slog.String("jobId", worker.Job.JobId))

	logger.Info("Waiting for job response")

	// reading from socket
	decoder := json.NewDecoder(conn)
	var response types.JobResponse
	if err := decoder.Decode(&response); err != nil {
		logger.Error("Failed to decode response", slog.String("error", err.Error()))
		return types.JobResponse{}, err
	}

	logger.Info("Job response received", slog.String("responseJobId", response.JobId), slog.String("responseStatus", response.Status))

	return response, nil
}

// WaitForUnFrezzing waits for the socket to be unfrezzing before returning.
func (worker *Worker) WaitForUnFrezzing() bool {
	// adding Read lock on the worker
	worker.mu.RLock()
	defer worker.mu.RUnlock()

	conn := worker.Conn
	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "worker.waitForUnFrezzing"))

	logger.Info("Waiting for the socket to be unfrezzing, and getting ready for job")

	// reading from socket
	decoder := json.NewDecoder(conn)
	var response types.UnFrezzingResponse
	if err := decoder.Decode(&response); err != nil {
		logger.Error("Failed to decode response", slog.String("error", err.Error()))
		return false
	}

	logger.Info("Job response received", slog.String("UnfrezzingResponseStatus", response.Status))
	if response.Status == "true" {
		return true
	}
	return false
}
