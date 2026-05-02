package ipc

import (
	"encoding/json"
	"log/slog"
	"net"

	// Alias the import to 'types'
	types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

type Worker struct {
	*types.Worker
}

// initConnection initializes a connection to the unix socket and returns it.
// NOTE: The caller is responsible for calling conn.Close() when done.
func initConnection(socketPath string, _logger *slog.Logger) net.Conn {

	logger := _logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.initConnection"))

	logger.Info("Connecting to socket", slog.String("socketPath", socketPath))
	// connecting to the unix socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		logger.Error("Failed to connect to socket", slog.String("error", err.Error()))
		return nil
	}
	logger.Info("Connected to socket", slog.String("socketPath", socketPath))

	return conn
}

// initJob sends the job over the connection
func (worker *Worker) initJob(job types.Job) {

	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.initJob"), slog.String("jobId", job.JobId))
	conn := worker.Conn

	logger.Info("Sending job")

	// making data into json format to send
	jsonData, err := json.Marshal(job)
	if err != nil {
		logger.Error("Failed to marshal JSON", slog.String("error", err.Error()))
		conn.Close() // Clean up the connection we just opened
		logger.Error("Failed to send job", slog.String("error", err.Error()))
		return
	}

	// writing to socket
	_, err = conn.Write(jsonData)
	if err != nil {
		logger.Error("Failed to write to socket", slog.String("error", err.Error()))
		conn.Close()
		logger.Error("Failed to send job", slog.String("error", err.Error()))
		return
	}

	logger.Info("Job sent successfully")

}

// waitForJobResponse waits for a job response from the socket and returns it.
func (worker *Worker) waitForJobResponse() types.JobResponse {
	conn := worker.Conn
	logger := worker.Logger.With(slog.String("type", "ipc"), slog.String("module", "ipc.waitForJobResponse"), slog.String("jobId", worker.Job.JobId))

	logger.Info("Waiting for job response")

	// reading from socket
	decoder := json.NewDecoder(conn)
	var response types.JobResponse
	if err := decoder.Decode(&response); err != nil {
		logger.Error("Failed to decode response", slog.String("error", err.Error()))
		return types.JobResponse{}
	}

	logger.Info("Job response received", slog.String("responseJobId", response.JobId), slog.String("responseStatus", response.Status))

	return response
}
