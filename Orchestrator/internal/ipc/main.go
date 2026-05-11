package ipc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	// Alias the import to 'types'
	types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

// InitConnection dials the unix socket at socketPath, retrying up to 10 times
// with 100ms between attempts (giving the worker process time to bind).
// Returns a net.Conn on success, or an error if all attempts fail.
// NOTE: The caller is responsible for calling conn.Close() when done.
func InitConnection(socketPath string, _logger *slog.Logger) (net.Conn, error) {

	logger := _logger.With(slog.String("type", "module"), slog.String("module", "ipc.initConnection"), slog.String("socketPath", socketPath))

	logger.Info("Connecting to socket")

	var (
		conn net.Conn
		err  error
	)
	// connecting to the unix socket
	for i := range 10 {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			logger.Info("Connected to socket")
			return conn, nil
		}
		logger.Debug("socket not ready, retrying", slog.Int("attempt", i+1), slog.Any("error", err))
		// sleep for 10 seconds
		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("ipc.InitConnection: failed after 10 attempts with 100 ms gaps on `%q` socket: %w", socketPath, err)
}

// InitJob serialises job to JSON and writes it to the worker's connection.
// It holds a write lock for the duration since it writes to the shared conn.
func InitJob(worker *types.Worker, job types.Job) error {

	//write lock to ensure thread safety
	worker.ConnMu.Lock()
	defer worker.ConnMu.Unlock() // unlock the write lock

	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.initJob"), slog.String("jobId", job.JobId))

	logger.Info("Sending job...")

	// making data into json format to send
	jsonData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("ipc.InitJob: marshal: %w", err)
	}

	if _, err = worker.Conn.Write(jsonData); err != nil {
		return fmt.Errorf("ipc.InitJob: write: %w", err)
	}

	logger.Info("Job sent successfully")
	return nil

}

// WaitForJobResponse blocks until the worker sends back a JobResponse.
// It holds a read lock since it only reads from the connection.
func WaitForJobResponse(worker *types.Worker) (types.JobResponse, error) {

	// adding Write lock on the worker for conn write only
	worker.ConnMu.RLock()
	defer worker.ConnMu.RUnlock()

	conn := worker.Conn
	logger := worker.Logger.With(
		slog.String("type", "ipc"),
		slog.String("module", "ipc.waitForJobResponse"),
		slog.String("jobId", worker.Job.JobId),
	)

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

// WaitForUnfreezing blocks until the worker signals it is ready to accept a new job.
// Returns an error if the decode fails or the worker reports it is not ready.
func WaitForUnfreezing(worker *types.Worker) error {
	worker.ConnMu.RLock()
	defer worker.ConnMu.RUnlock()

	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "worker.waitForUnFrezzing"))

	logger.Info("Waiting for the socket to be unfrezzing, and getting ready for job")

	// reading from socket
	var response types.UnFreezingResponse
	if err := json.NewDecoder(worker.Conn).Decode(&response); err != nil {
		return fmt.Errorf("ipc.WaitForUnfreezing: decode: %w", err)
	}

	if !response.Ready {
		return fmt.Errorf("ipc.WaitForUnfreezing: worker reported not ready")
	}

	logger.Info("worker is ready")
	return nil
}
