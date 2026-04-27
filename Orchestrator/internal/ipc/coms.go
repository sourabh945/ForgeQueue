package ipc

import (
	"encoding/json"
	"log/slog"
	"net"

	"github.com/sourabh945/ForgeQueue/Orchestrator/internal/types/ipc"
)

// this function take a
func connect_send_job(socketPath string, job ipc.Job, logger slog.Logger) net.Conn {

	//setup the logger
	logger.With(slog.String("module", "ipc"), job.JobId)

	// connecting to the unix socket
	conn, err := net.Dial("unix", socketPath)

	if err != nil {
		logger.Error("Failed to connect: %v\n", err)
		return nil
	}

	//making data into json format to send
	jsonData, _ := json.Marshal(job)

	//writting to socket
	conn.Write(jsonData)

	return conn

}
