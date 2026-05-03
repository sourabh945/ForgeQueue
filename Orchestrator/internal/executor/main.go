package executor

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	Types "github.com/sourabh945/ForgeQueue/Orchestrator/internal/types"
)

func InitWorkerProcess(cmd []string, _logger *slog.Logger, workerCount int) Worker {

	// making the socket path and worker ID
	socketPath := fmt.Sprintf("/tmp/forgequeue-worker-%d.sock", workerCount)

	id := fmt.Sprintf("worker-%d", workerCount)

	// creating the module logger and worker-specific logger
	moduleLogger := _logger.With(slog.String("type", "orch.module"), slog.String("module", "InitWorkerProcess"))

	logger := moduleLogger.With(slog.String("id", id))

	// removing any existing socket file (cleanup)
	os.Remove(socketPath)

	// creating the command to execute
	_cmd := append(cmd, socketPath)

	// executing the command
	commnad := exec.Command(_cmd[0], _cmd[1:]...)

	// setting the process group ID to the worker's PID
	commnad.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// getting the stdout and stderr pipes
	stdoutPipe, err := commnad.StdoutPipe()
	if err != nil {
		moduleLogger.Error("failed to create stdout pipe", "error", err)
		return Worker{}
	}

	stderrPipe, err := commnad.StderrPipe()
	if err != nil {
		moduleLogger.Error("failed to create stderr pipe", "error", err)
		return Worker{}
	}

	return Worker{
		Worker: &Types.Worker{
			ID:         id,
			Status:     "free",
			SocketPath: socketPath,
			Cmd:        commnad,
			Logger:     logger,
			StdOut:     stdoutPipe,
			StdErr:     stderrPipe,
		},
	}
}
